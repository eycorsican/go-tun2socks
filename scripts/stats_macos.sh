pid=`ps -A | grep -m1 tun2socks | awk '{print $1}'`
sudo kill -s SIGUSR1 $pid
