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

(as root)

```
cd /etc/systemd/system/
ln -s /deploy/ai-things/systemd/tortoise/tortoise-wave.service
systemctl daemon-reload
systemctl enable --now tortoise-wave.service
```
