### 🔖 主要功能

+ 管理 Digital Ocean 账号，查看账号余额，批量测试账号
+ 创建和管理虚拟机

### 🪂 环境预设

```
apt install python3 git -y
apt install python3-pip -y
rm /usr/lib/python3.11/EXTERNALLY-MANAGED
```

### 😻 用法

```
git clone https://github.com/xkatld/digitalocean-tgbot.git
cd digitalocean-tgbot
pip3 install -r requirements.txt
#编辑 `config.json`，填入相关配置
python3 main.py
```
1. `git clone` 项目至本地，`pip3 install -r requirements.txt` 安装依赖
2. 在 [@botfather](https://t.me/botfather) 新建一个 `bot`，记下 `token`
3. 编辑 `config.json`，填入相关配置
4. `python3 main.py` 运行 `bot`

5.强制设置开机密码修改'utils/set_root_password_script.py',{password}改为需要设置的密码就行
