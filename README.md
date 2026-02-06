# DigitalOcean Telegram Bot

DigitalOcean 账号余额与资源监控 Telegram 机器人。

## 展示

配置
~~~
BOT:
  NAME: "DigitalOcean TGbot"
  # 获取@BotFather
  TOKEN: ""
  # 获取@get_id_bot
  ADMINS: [""]
~~~ 

## 部署

```bash
# 下载 linux-amd64 版本并运行
curl -L -o Linux-amd64.tar.gz $(curl -s https://api.github.com/repos/xkatld/digitalocean-tgbot/releases/latest | grep "browser_download_url" | grep "Linux-amd64.tar.gz" | cut -d '"' -f 4)
tar -xzf Linux-amd64.tar.gz
chmod +x Linux-amd64
# 修改 config.yaml 后运行
nohup ./Linux-amd64 > bot.log 2>&1 &
```

## 协议

采用 [CC BY-NC 4.0](LICENSE) 协议。

## 声明

[重要] **禁止商用**。
[注意] **必须署名**。

本项目仅用于个人学习与研究，未经许可不得用于商业盈利。