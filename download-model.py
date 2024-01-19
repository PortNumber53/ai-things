import argparse
from huggingface_hub import snapshot_download

def download_model(repo_id, use_auth_token=True):
    downloaded_model_path = snapshot_download(
        repo_id=repo_id,
        use_auth_token=use_auth_token
    )
    print(downloaded_model_path)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Download model from Hugging Face Hub.")
    parser.add_argument("--repo_id", required=True, help="Hugging Face repository ID")

    args = parser.parse_args()
    download_model(repo_id=args.repo_id)
