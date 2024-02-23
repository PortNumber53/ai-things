import argparse
import sys
import os
import json
import pika
from dotenv import load_dotenv
import torch
import torchaudio
from api import TextToSpeech, MODELS_DIR
from utils.audio import load_voices
import time
import signal

# Global variable to track whether a job is currently being processed
job_processing = False

def signal_handler(sig, frame):
    global job_processing
    if job_processing:
        print("Waiting for the current job to finish...")
        # Wait until the job finishes before exiting
        while job_processing:
            time.sleep(1)
    print("Exiting...")
    sys.exit(0)

# Register the signal handler for SIGINT (CTRL+C) and SIGTERM (kill)
signal.signal(signal.SIGINT, signal_handler)
signal.signal(signal.SIGTERM, signal_handler)


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

        # Add heartbeat parameter to the connection parameters
        url_params.heartbeat = 600

        connection = pika.BlockingConnection(url_params)

        channel = connection.channel()
        channel.queue_declare(queue=queue_name)
        channel.basic_publish(exchange='', routing_key=queue_name, body=json.dumps(message))
        print(f"Message pushed to {queue_name} queue: {message}")
    except Exception as e:
        print(f"Error pushing message to {queue_name} queue: {e}")
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
    job = json.loads(body.decode())
    process_job(job)

def process_job(job):
    global job_processing
    job_processing = True

    start_time = time.time()

    # Default values fetched from .env or original argument parser
    default_args = {
        'text': get_default_arg('text', "The expressiveness of autoregressive transformers is literally nuts! I absolutely adore them."),
        'voice': get_default_arg('voice', 'random'),
        'preset': get_default_arg('preset', 'fast'),
        'use_deepspeed': get_default_arg('use_deepspeed', False),
        'kv_cache': get_default_arg('kv_cache', True),
        'half': get_default_arg('half', True),
        'output_path': get_default_arg('output_path', 'results/'),
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

    if torch.backends.mps.is_available():
        args.use_deepspeed = False
    os.makedirs(args.output_path, exist_ok=True)
    tts = TextToSpeech(models_dir=args.model_dir, use_deepspeed=args.use_deepspeed, kv_cache=args.kv_cache, half=args.half)

    selected_voices = args.voice.split(',')
    for k, selected_voice in enumerate(selected_voices):
        if '&' in selected_voice:
            voice_sel = selected_voice.split('&')
        else:
            voice_sel = [selected_voice]
        voice_samples, conditioning_latents = load_voices(voice_sel)

        gen, dbg_state = tts.tts_with_preset(args.text, k=args.candidates, voice_samples=voice_samples, conditioning_latents=conditioning_latents,
                                  preset=args.preset, use_deterministic_seed=args.seed, return_deterministic_state=True, cvvp_amount=args.cvvp_amount)
        if isinstance(gen, list):
            for j, g in enumerate(gen):
                filename = args.filename if args.filename else f'{selected_voice}_{k}_{j}'
                torchaudio.save(os.path.join(args.output_path, f'{filename}.wav'), g.squeeze(0).cpu(), 24000)
                print(f"File saved: {filename}.wav")

                end_time = time.time()
                processing_time = end_time - start_time

                message = {
                    'filename': filename,
                    'text': args.text,
                    'selected_voice': selected_voice,
                    'processing_time': processing_time,
                }

                push_message_to_queue('wav_to_mp3', message)

        else:
            filename = args.filename if args.filename else f'{selected_voice}_{k}'
            torchaudio.save(os.path.join(args.output_path, f'{filename}.wav'), gen.squeeze(0).cpu(), 24000)
            print(f"File saved: {filename}.wav")

            # Measure processing time
            end_time = time.time()
            processing_time = end_time - start_time

            message = {
                'filename': filename,
                'text': args.text,
                'selected_voice': selected_voice,
                'processing_time': processing_time,
            }

            push_message_to_queue('wav_to_mp3', message)


        if args.produce_debug_state:
            os.makedirs('debug_states', exist_ok=True)
            torch.save(dbg_state, f'debug_states/do_tts_debug_{filename}.pth')
    sys.stdout.flush()
    job_processing = False


if __name__ == '__main__':
    load_dotenv()  # Load variables from .env file

    parser = argparse.ArgumentParser()
    parser.add_argument('--rabbitmq_url', type=str, help='RabbitMQ server URL.', default=f"amqp://{os.getenv('RABBITMQ_USER')}:{os.getenv('RABBITMQ_PASSWORD')}@{os.getenv('RABBITMQ_HOST')}:{os.getenv('RABBITMQ_PORT')}/{os.getenv('RABBITMQ_VHOST')}")
    args = parser.parse_args()

    connection = None
    channel = None
    try:
        url_params = pika.URLParameters(args.rabbitmq_url)

        # Add heartbeat parameter to the connection parameters
        url_params.heartbeat = 600

        connection = pika.BlockingConnection(url_params)
        channel = connection.channel()
        channel.queue_declare(queue='tts_wave', durable=True)
        channel.basic_consume(queue='tts_wave', on_message_callback=callback, auto_ack=True)
        print('Waiting for messages. To exit press CTRL+C')
        channel.start_consuming()
    except pika.exceptions.AMQPConnectionError as e:
        print(f"Error connecting to RabbitMQ server: {e}")
    except KeyboardInterrupt:
        print("Exiting...")
    finally:
        if channel:
            channel.close()
        if connection and connection.is_open:
            connection.close()
