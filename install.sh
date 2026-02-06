ARCH=$(uname -m)
case "$ARCH" in
    x86_64) T="amd64" ;;
    aarch64|arm64) T="arm64" ;;
    *) echo "[错误] 不支持的架构: $ARCH"; exit 1 ;;
esac

FILE="Linux-$T.tar.gz"
URL=$(curl -s https://api.github.com/repos/xkatld/digitalocean-tgbot/releases/latest | grep "browser_download_url" | grep "$FILE" | cut -d '"' -f 4)

if [ -z "$URL" ]; then
    echo "[错误] 无法获取下载地址，请检查网络"
    exit 1
fi

pkill Linux- 2>/dev/null

[ -f config.yaml ] && cp config.yaml config.yaml.bak

curl -L -o "$FILE" "$URL"
tar -xzf "$FILE"
chmod +x "Linux-$T"

if [ -f config.yaml.bak ]; then
    mv config.yaml.bak config.yaml
    echo "[正确] 已恢复现有配置文件"
else
    if grep -q "__TOKEN__" config.yaml; then
        echo ">>> 检测到首次安装，开始配置环境:"
        read -p "请输入 Bot Token: " T_VAL
        read -p "请输入管理员ID: " A_VAL
        sed -i "s/__TOKEN__/$T_VAL/g" config.yaml
        sed -i "s/__ADMINS__/$A_VAL/g" config.yaml
        echo "[正确] 配置文件已更新"
    fi
fi

nohup "./Linux-$T" > bot.log 2>&1 &
echo "[正确] 服务已在后台启动"
