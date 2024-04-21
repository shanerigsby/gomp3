Utility, i.e. "Why would I want to use this?"  
It's a simple web api. You send a POST request with a body value of a YT url (string). The service creates an mp3 of the video's audio and returns a link to it.  
  
View code comments for available arguments. Default values must be defined in a config.json file if they are not explicitly given.  
  
To create as a systemd service, create a file in /etc/systemd/service/gomp3.service (this assumes the executable and service are called gomp3):  
  
[Unit]  
Description=gomp3 youtube audio  
After=network.target  
  
[Service]  
User=someuser
ExecStart=/home/someuser/source/gomp3/gomp3 -c /home/someuser/source/gomp3/config.json  
Restart=always  
  
[Install]  
WantedBy=multi-user.target  

...  
start up the service and enable it to run on startup:  
sudo systemctl daemon-reload  
sudo systemctl start gomp3.service  
sudo systemctl enable gomp3.service  

check the recent logs:  
journalctl -u gomp3.service -n 20 -r


TODO:
work out kinks in command line args. they should all be optional
