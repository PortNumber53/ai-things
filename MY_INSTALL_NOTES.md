# My Install Notes

To get pytorch to use my RTX 3050 on my Lenovo Ideapad 5 (nVidia + AMDGPU)

```
pip3 install torch==2.2.0+cu121 -f https://download.pytorch.org/whl/torch_stable.html
```

## Manual deploment notes

```
sudo mkdir -pv /deploy/ai-things/
sudo chown -Rv grimlock:grimlock /deploy

```

## set up systemd service

install conda

```
yay -S wget
yay -S python-pluggy python-pycosat python-ruamel-yaml anaconda


git clone https://aur.archlinux.org/python-conda-package-handling.git && cd python-conda-package-handling
makepkg -is

git clone https://aur.archlinux.org/python-conda.git && cd python-conda
makepkg -is

conda --version

wget https://github.com/conda-forge/miniforge/releases/latest/download/Miniforge3-Linux-x86_64.sh
./Miniforge3-Linux-x86_64.sh


```

(as root)

```
cd /etc/systemd/system/
ln -s /deploy/ai-things/systemd/tortoise/tortoise-wave.service
systemctl daemon-reload
systemctl enable --now tortoise-wave.service
```

## Tortoise folder

```
conda create -n tortoise python=3.11
conda activate tortoise

cd tortoise-tts
pip install -r requirements.txt
pip install git+https://github.com/neonbjb/tortoise-tts
pip install pika python-dotenv torch torchaudio psycopg2 progressbar
```

## Conda installation

Based off: https://www.jeremymorgan.com/tutorials/python-tutorials/how-to-install-anaconda-arch-linux/

```
yay -S python-pluggy python-pycosat python-ruamel-yaml

git clone https://aur.archlinux.org/python-conda-package-handling.git && cd python-conda-package-handling
makepkg -is

git clone https://aur.archlinux.org/python-conda.git && cd python-conda
makepkg -is

conda config --set report_errors true
```

Torroise-TTS

```

pip install -r requirements.txt
pip install git+https://github.com/neonbjb/tortoise-tts

```

## Folders:

```
sudo mkdir -pv /output/funfacts/
sudo chown -Rv grimlock:grimlock /output/funfacts/
```

## Subtitles

```
conda create -n subtitle python=3.11
conda activate subtitle
pip install -r requirements.txt



```
