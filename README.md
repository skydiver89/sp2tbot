# Description  
This telegram bot can translate audio and voice messages to text.  

# Build  
You'll need go compiler and make  
Simply run:

```
make
```

# Prerequisites  
VOSK library is built for linux x64. If you need another OS/architechture, build it yourself.  

## VOSK model  
Download and unzip model for your language https://alphacephei.com/vosk/models  
Keep in mind that the larger the model, the better it will translate and the more RAM it will use.  

## Config file  
You should rename config.yaml.example to config.example. Then edit config file. Your telegram ID you can detect in this bot https://t.me/my_id_bot. Register your bot and get API token at https://t.me/BotFather.  

# Usage  

## Run  
Simply run:  

```
make run
```

## Systemd  
If you need systemd service rename sp2tbot.service.example to sp2tbot.service and edit it. Then move it to /etc/systemd/system. Then run:  

```
sudo systemctl daemon-reload
```

To start bot, run:  

```
sudo systemctl start sp2tbot.service
```

If you need autostart, run:  

```
sudo systemctl enable sp2tbot.service
```

To stop bot, run:  

```
sudo systemctl stop sp2tbot.service
```

To disable autostart, run:  

```
sudo systemctl disable sp2tbot.service
```
