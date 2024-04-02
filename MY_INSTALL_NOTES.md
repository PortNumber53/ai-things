# My Install Notes



## packages
```
yay -S php php-pgsql php-gd redis php-redis
```

## install conda

```
yay -S wget
yay -S python-pluggy python-pycosat python-ruamel-yaml  python-pluggy python-pycosat python-ruamel-yaml



git clone https://aur.archlinux.org/python-conda-package-handling.git && cd python-conda-package-handling
makepkg -is

git clone https://aur.archlinux.org/python-conda.git && cd python-conda
makepkg -is

conda --version

wget https://github.com/conda-forge/miniforge/releases/latest/download/Miniforge3-Linux-x86_64.sh
./Miniforge3-Linux-x86_64.sh


```


To get pytorch to use my RTX 3050 on my Lenovo Ideapad 5 (nVidia + AMDGPU)

```
yay -S nvidia-dkms nvidia-utils
# blacklist nouveau

nano /etc/modprobe.d/backlist.conf
blacklist nouveau
blacklist rivafb
blacklist nvidiafb
blacklist rivatv
blacklist nv
blacklist uvcvideo



# mkinitcpio -p linux

./webui.sh --listen --api --lowvram --xformers

```

## Speech Conda environment:

```
conda create -n speech python=3.11

conda activate speech

pip install torch==2.2.0+cu121 -f https://download.pytorch.org/whl/torch_stable.html

pip install nvitop psutil pynvml cachetools nvidia-ml-py termcolor


yay -S ffmpeg sox
```


## Manual deploment notes

```
sudo mkdir -pv /deploy/ai-things/
sudo chown -Rv grimlock:grimlock /deploy

```

## set up systemd service

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
sudo mkdir -p /output/{funfacts,mp3,results,subtitles,waves}
sudo chown -Rv grimlock:grimlock /output/

sudo mkdir -pv /storage/ai/
sudo chown -Rv grimlock:grimlock /storage
```

## Subtitles

```
conda create -n subtitle python=3.11
conda activate subtitle
pip install -r requirements.txt



```




###

```
conda create -n speech python=3.11

pip install piper-tts
```

```
sudo mkdir -pv /storage/ai/
sudo chown -Rv grimlock:grimlock /storage



sudo mkdir -p /deploy/ai-things
sudo chown -Rv grimlock:grimlock /deploy/

sudo mkdir -pv sudo mkdir -pv /output/waves /output/mp3 /output/subtitles /output/results /output/funfacts
sudo chown -Rv grimlock:grimlock /output/


```




## PHP Notes

```
/etc/php/conf.d/sockets.ini
extension=sockets

/etc/php/conf.d/iconv.ini
extension=iconv

/etc/php/conf.d/pdo_pgsql.ini
extension=pdo_pgsql


```
