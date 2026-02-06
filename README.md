# DigitalOcean Telegram Bot

DigitalOcean 账号余额与资源监控 Telegram 机器人。

## 展示

~~~
BOT:
  NAME: "DigitalOcean TGbot"
  TOKEN: ""
  ADMINS: ""
~~~ 

## 部署

```bash
ARCH=$(uname -m) && [ "$ARCH" = "x86_64" ] && T="amd64" || T="arm64"
FILE="Linux-$T.tar.gz"
URL=$(curl -s https://api.github.com/repos/xkatld/digitalocean-tgbot/releases/latest | grep "browser_download_url" | grep "$FILE" | cut -d '"' -f 4)
curl -L -o $FILE $URL && tar -xzf $FILE && chmod +x Linux-$T
nohup ./Linux-$T > bot.log 2>&1 &
```

## 升级

```bash
pkill Linux- && ARCH=$(uname -m) && [ "$ARCH" = "x86_64" ] && T="amd64" || T="arm64" && FILE="Linux-$T.tar.gz" && URL=$(curl -s https://api.github.com/repos/xkatld/digitalocean-tgbot/releases/latest | grep "browser_download_url" | grep "$FILE" | cut -d '"' -f 4) && curl -L -o $FILE $URL && tar -xzf $FILE && chmod +x Linux-$T && nohup ./Linux-$T > bot.log 2>&1 &
```

## 协议

采用 [CC BY-NC 4.0](LICENSE) 协议。

## 声明

[重要] **禁止商用**。
[注意] **必须署名**。

本项目仅用于个人学习与研究，未经许可不得用于商业盈利。