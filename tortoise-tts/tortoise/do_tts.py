import argparse
import os
import json
import pika
from dotenv import load_dotenv
import torch
import torchaudio
from api import TextToSpeech, MODELS_DIR
from utils.audio import load_voices

def get_default_arg(name, default):
    # Fetch value from .env if available, otherwise use default value
    return os.getenv(name.upper(), default)

def callback(ch, method, properties, body):
    job = json.loads(body.decode())
    process_job(job)

def process_job(job):
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
        'cvvp_amount': get_default_arg('cvvp_amount', 0.0)
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
                torchaudio.save(os.path.join(args.output_path, f'{selected_voice}_{k}_{j}.wav'), g.squeeze(0).cpu(), 24000)
                print(f"File saved: {selected_voice}_{k}_{j}.wav")
        else:
            torchaudio.save(os.path.join(args.output_path, f'{selected_voice}_{k}.wav'), gen.squeeze(0).cpu(), 24000)
            print(f"File saved: {selected_voice}_{k}.wav")

        if args.produce_debug_state:
            os.makedirs('debug_states', exist_ok=True)
            torch.save(dbg_state, f'debug_states/do_tts_debug_{selected_voice}.pth')

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
        channel.queue_declare(queue='tts_wave')
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



    # parser = argparse.ArgumentParser()
    # parser.add_argument('--text', type=str, help='Text to speak.', default="The expressiveness of autoregressive transformers is literally nuts! I absolutely adore them.")
    # parser.add_argument('--voice', type=str, help='Selects the voice to use for generation. See options in voices/ directory (and add your own!) '
    #                                              'Use the & character to join two voices together. Use a comma to perform inference on multiple voices.', default='random')
    # parser.add_argument('--preset', type=str, help='Which voice preset to use.', default='fast')
    # parser.add_argument('--use_deepspeed', type=str, help='Which voice preset to use.', default=False)
    # parser.add_argument('--kv_cache', type=bool, help='If you disable this please wait for a long a time to get the output', default=True)
    # parser.add_argument('--half', type=bool, help="float16(half) precision inference if True it's faster and take less vram and ram", default=True)
    # parser.add_argument('--output_path', type=str, help='Where to store outputs.', default='results/')
    # parser.add_argument('--model_dir', type=str, help='Where to find pretrained model checkpoints. Tortoise automatically downloads these to .models, so this'
    #                                                   'should only be specified if you have custom checkpoints.', default=MODELS_DIR)
    # parser.add_argument('--candidates', type=int, help='How many output candidates to produce per-voice.', default=3)
    # parser.add_argument('--seed', type=int, help='Random seed which can be used to reproduce results.', default=None)
    # parser.add_argument('--produce_debug_state', type=bool, help='Whether or not to produce debug_state.pth, which can aid in reproducing problems. Defaults to true.', default=True)
    # parser.add_argument('--cvvp_amount', type=float, help='How much the CVVP model should influence the output.'
    #                                                       'Increasing this can in some cases reduce the likelihood of multiple speakers. Defaults to 0 (disabled)', default=.0)
    # args = parser.parse_args()
    # if torch.backends.mps.is_available():
    #     args.use_deepspeed = False
    # os.makedirs(args.output_path, exist_ok=True)
    # tts = TextToSpeech(models_dir=args.model_dir, use_deepspeed=args.use_deepspeed, kv_cache=args.kv_cache, half=args.half)

    # selected_voices = args.voice.split(',')
    # for k, selected_voice in enumerate(selected_voices):
    #     if '&' in selected_voice:
    #         voice_sel = selected_voice.split('&')
    #     else:
    #         voice_sel = [selected_voice]
    #     voice_samples, conditioning_latents = load_voices(voice_sel)

    #     gen, dbg_state = tts.tts_with_preset(args.text, k=args.candidates, voice_samples=voice_samples, conditioning_latents=conditioning_latents,
    #                               preset=args.preset, use_deterministic_seed=args.seed, return_deterministic_state=True, cvvp_amount=args.cvvp_amount)
    #     if isinstance(gen, list):
    #         for j, g in enumerate(gen):
    #             torchaudio.save(os.path.join(args.output_path, f'{selected_voice}_{k}_{j}.wav'), g.squeeze(0).cpu(), 24000)
    #     else:
    #         torchaudio.save(os.path.join(args.output_path, f'{selected_voice}_{k}.wav'), gen.squeeze(0).cpu(), 24000)

    #     if args.produce_debug_state:
    #         os.makedirs('debug_states', exist_ok=True)
    #         torch.save(dbg_state, f'debug_states/do_tts_debug_{selected_voice}.pth')
