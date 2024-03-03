import argparse
import sys
import os
import json
import pika
from dotenv import load_dotenv
import torch
import torchaudio
import psycopg2
from psycopg2 import sql
from api import TextToSpeech, MODELS_DIR
from utils.audio import load_voices
import time
import signal
import logging

# Set up logging
logging.basicConfig(level=logging.WARNING)
logger = logging.getLogger(__name__)

# Global variable to track whether a job is currently being processed
job_processing = False

# Set the base output folder
BASE_OUTPUT_FOLDER = os.getenv('BASE_OUTPUT_FOLDER', '/output/')

def signal_handler(sig, frame):
    global job_processing
    if job_processing:
        logger.info("Waiting for the current job to finish...")
        # Wait until the job finishes before exiting
        while job_processing:
            time.sleep(1)
    logger.info("Exiting...")
    raise KeyboardInterrupt

# Register the signal handler for SIGINT (CTRL+C) and SIGTERM (kill)
signal.signal(signal.SIGINT, signal_handler)
signal.signal(signal.SIGTERM, signal_handler)

# Function to fetch PostgreSQL connection
def get_postgres_connection():
    try:
        conn = psycopg2.connect(
            dbname=os.getenv('DB_DATABASE'),
            user=os.getenv('DB_USERNAME'),
            password=os.getenv('DB_PASSWORD'),
            host=os.getenv('DB_HOST'),
            port=os.getenv('DB_PORT')
        )
        return conn
    except psycopg2.Error as e:
        logger.error(f"Error connecting to PostgreSQL: {e}")
        return None


def push_message_to_queue(queue_name, message):
    global job_processing
    job_processing = True

    parser = argparse.ArgumentParser()
    parser.add_argument('--rabbitmq_url', type=str, help='RabbitMQ server URL.', default=f"amqp://{os.getenv('RABBITMQ_USER')}:{os.getenv('RABBITMQ_PASSWORD')}@{os.getenv('RABBITMQ_HOST')}:{os.getenv('RABBITMQ_PORT')}/{os.getenv('RABBITMQ_VHOST')}")
    args = parser.parse_args()

    connection = None
    channel = None

    try:
        url_params = pika.URLParameters(args.rabbitmq_url)
        url_params.heartbeat = 600
        pika_logger = logging.getLogger('pika')
        pika_logger.setLevel(logging.ERROR)        # Add heartbeat parameter to the connection parameters

        connection = pika.BlockingConnection(url_params)
        channel = connection.channel()

        # Check if the channel is still open, if not, reopen it
        if not channel.is_open:
            channel = connection.channel()

        channel.queue_declare(queue=queue_name)
        channel.basic_publish(exchange='', routing_key=queue_name, body=json.dumps(message))
        logger.info(f"Message pushed to {queue_name} queue: {message}")
    except Exception as e:
        logger.error(f"Error pushing message to {queue_name} queue: {e}")
    finally:
        if channel:
            channel.close()
        if connection and connection.is_open:
            connection.close()

    job_processing = False


def get_default_arg(name, default):
    # Fetch value from .env if available, otherwise use default value
    return os.getenv(name.upper(), default)

def callback(ch, method, properties, body):
    try:
        job = json.loads(body.decode())
        process_job(job)
        ch.basic_ack(delivery_tag=method.delivery_tag)  # Acknowledge message after processing
    except Exception as e:
        logger.error(f"Error processing message: {e}")


def update_postgres_meta(content_id, filename, sentence_id=None):
    conn = get_postgres_connection()
    if conn is None:
        return

    try:
        cur = conn.cursor()
        # Fetch existing meta JSON
        cur.execute(sql.SQL("SELECT meta FROM contents WHERE id = %s"), (content_id,))
        row = cur.fetchone()
        if row:
            existing_meta = row[0] or {}  # Handle None case
            # Check if filename already exists in meta
            if "filenames" in existing_meta and any(entry["filename"] == f'{filename}.wav' for entry in existing_meta["filenames"]):
                logger.warning(f"Filename {filename} already exists in meta for content_id {content_id}")
                return
            # Update meta with new filename
            existing_meta.setdefault("filenames", []).append({"filename": f'{filename}.wav', "sentence_id": sentence_id})
            # Update the contents table with the new meta
            cur.execute(sql.SQL("UPDATE contents SET meta = %s WHERE id = %s"),
                        (json.dumps(existing_meta), content_id))
            conn.commit()
            logger.info(f"Meta updated for content_id {content_id} with filename {filename}")
        else:
            logger.error(f"No row found for content_id {content_id}")
    except psycopg2.Error as e:
        logger.error(f"Error updating PostgreSQL meta: {e}")
    finally:
        if conn:
            conn.close()



def process_job(job):
    global job_processing
    job_processing = True

    start_time = time.time()

    # Fetch default arguments
    default_args = {
        'text': get_default_arg('text', "The expressiveness of autoregressive transformers is literally nuts! I absolutely adore them."),
        'voice': get_default_arg('voice', 'random'),
        'preset': get_default_arg('preset', 'fast'),
        'use_deepspeed': get_default_arg('use_deepspeed', False),
        'kv_cache': get_default_arg('kv_cache', True),
        'half': get_default_arg('half', True),
        'output_path': get_default_arg('output_path', BASE_OUTPUT_FOLDER),
        'model_dir': get_default_arg('model_dir', MODELS_DIR),
        'candidates': get_default_arg('candidates', 1),
        'seed': get_default_arg('seed', None),
        'produce_debug_state': get_default_arg('produce_debug_state', True),
        'cvvp_amount': get_default_arg('cvvp_amount', 0.0),
        'filename': None
    }

    # Override default values with job arguments
    for arg_name, arg_value in job.items():
        default_args[arg_name] = arg_value

    args = argparse.Namespace(**default_args)
    filename = args.filename if args.filename else f'{selected_voice}.wav'  # Add ".wav" extension
    print(f'FILENAME TO PROCESS: {filename}')

    # Check if wave information already exists in meta JSON
    content_id = job.get("content_id")
    if content_id:
        conn = get_postgres_connection()
        if conn is not None:
            try:
                cur = conn.cursor()
                cur.execute(sql.SQL("SELECT meta FROM contents WHERE id = %s"), (content_id,))
                row = cur.fetchone()
                if row:
                    existing_meta = row[0] or {}
                    if "filenames" in existing_meta:
                        existing_filenames = [entry.get("filename", "") for entry in existing_meta["filenames"]]
                        print(existing_filenames)
                        # Check if the current filename matches any existing filename
                        if filename in existing_filenames:
                            logger.warn(f"Wave information for {filename} already exists in meta for content_id {content_id}. Skipping processing.")
                            job_processing = False
                            return
            except psycopg2.Error as e:
                logger.error(f"Error fetching existing meta from PostgreSQL: {e}")
            finally:
                conn.close()

    if torch.backends.mps.is_available():
        args.use_deepspeed = False

    args.output_path = os.path.join(args.output_path, 'waves')
    os.makedirs(os.path.join(BASE_OUTPUT_FOLDER, args.output_path), exist_ok=True)

    # Initialize TextToSpeech instance
    tts = TextToSpeech(models_dir=args.model_dir, use_deepspeed=args.use_deepspeed, kv_cache=args.kv_cache, half=args.half)

    selected_voice = args.voice
    voice_samples, conditioning_latents = load_voices([selected_voice])

    # Perform text-to-speech synthesis
    gen, dbg_state = tts.tts_with_preset(args.text, k=args.candidates, voice_samples=voice_samples, conditioning_latents=conditioning_latents,
                              preset=args.preset, use_deterministic_seed=args.seed, return_deterministic_state=True, cvvp_amount=args.cvvp_amount)

    file_path = os.path.join(args.output_path, f'{filename}')

    # Save generated audio file
    try:
        torchaudio.save(file_path, gen.squeeze(0).cpu(), 24000)
        logger.warning(f"File saved: {filename}")
    except Exception as e:
        logger.error(f"Error saving file {filename}: {e}")

    # Measure processing time
    end_time = time.time()
    processing_time = end_time - start_time

    # Prepare message for queue
    message = {
        'filename': filename,
        'text': args.text,
        'selected_voice': selected_voice,
        'processing_time': processing_time,
    }

    # Push message to queue
    push_message_to_queue('wav_to_mp3', message)

    # Update PostgreSQL meta with filename
    content_id = job.get("content_id")
    sentence_id = job.get("sentence_id")
    if content_id:
        update_postgres_meta(content_id, filename, sentence_id)

    # Save debug state if required
    if args.produce_debug_state:
        os.makedirs('debug_states', exist_ok=True)
        try:
            torch.save(dbg_state, f'debug_states/do_tts_debug_{filename}.pth')
        except Exception as e:
            logger.error(f"Error saving debug state for {filename}: {e}")

    sys.stdout.flush()
    job_processing = False


def main():
    load_dotenv()  # Load variables from .env file

    parser = argparse.ArgumentParser()
    parser.add_argument('--rabbitmq_url', type=str, help='RabbitMQ server URL.', default=f"amqp://{os.getenv('RABBITMQ_USER')}:{os.getenv('RABBITMQ_PASSWORD')}@{os.getenv('RABBITMQ_HOST')}:{os.getenv('RABBITMQ_PORT')}/{os.getenv('RABBITMQ_VHOST')}")
    args = parser.parse_args()

    url_params = pika.URLParameters(args.rabbitmq_url)
    url_params.heartbeat = 600

    connection = None
    channel = None

    while True:
        try:
            connection = pika.BlockingConnection(url_params)
            channel = connection.channel()
            channel.queue_declare(queue='tts_wave', durable=True)
            channel.basic_qos(prefetch_count=1)  # Set prefetch count to 1
            channel.basic_consume(queue='tts_wave', on_message_callback=callback)
            logger.info('Waiting for messages. To exit press CTRL+C')
            channel.start_consuming()
        except pika.exceptions.AMQPConnectionError as e:
            logger.error(f"Error connecting to RabbitMQ server: {e}")
            logger.info("Attempting to reconnect in 5 seconds...")
            time.sleep(5)
        except KeyboardInterrupt:
            logger.info("Exiting...")
            break
        finally:
            if channel and channel.is_open:
                channel.close()
            if connection and connection.is_open:
                connection.close()

if __name__ == '__main__':
    main()
