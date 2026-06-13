#!/bin/bash
# ============================================================
# init_server.sh — 阿里云 ECS 首次登录一键安全加固
# 用法：ssh ecs-user@<公网IP> 后执行：
#   git clone <仓库地址> ~/arbitrage-system
#   cd ~/arbitrage-system && bash scripts/init_server.sh
# ============================================================
set -euo pipefail

echo "========================================="
echo " arbitrage-system — 服务器初始化"
echo " 执行时间：$(date '+%Y-%m-%d %H:%M:%S')"
echo "========================================="

# ---- Step 1: 系统更新 + 基础工具 ----
echo "[1/11] 更新系统并安装基础工具..."
sudo apt update && sudo apt upgrade -y
sudo apt install -y curl wget git vim htop ufw fail2ban net-tools

# ---- Step 2: Docker 安装 ----
echo "[2/11] 安装 Docker CE + Docker Compose v2..."
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"
echo "  ✅ Docker 已安装，ecs-user 已加入 docker 组"

# ---- Step 3: SSH 加固 ----
echo "[3/11] SSH 安全加固..."
SSH_PORT="${SSH_PORT:-22922}"
sudo sed -i "s/^#Port 22/Port $SSH_PORT/" /etc/ssh/sshd_config
sudo sed -i 's/^#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
sudo sed -i 's/^#PermitRootLogin prohibit-password/PermitRootLogin no/' /etc/ssh/sshd_config
echo "  ✅ SSH 端口: $SSH_PORT, 密码登录: 禁用, root 登录: 禁用"

# ---- Step 4: fail2ban ----
echo "[4/11] 配置 fail2ban..."
sudo tee /etc/fail2ban/jail.local > /dev/null <<EOF
[sshd]
enabled = true
port = $SSH_PORT
maxretry = 3
bantime = 3600
findtime = 600
EOF
sudo systemctl enable fail2ban
sudo systemctl restart fail2ban
echo "  ✅ fail2ban 已启用（3 次失败封 1 小时）"

# ---- Step 5: ufw 防火墙 ----
echo "[5/11] 配置 ufw 主机防火墙..."
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow "$SSH_PORT/tcp"
sudo ufw allow 41641/udp  # Tailscale
sudo ufw --force enable
echo "  ✅ ufw 已启用"

# ---- Step 6: 时区 + NTP ----
echo "[6/11] 设置时区 + NTP..."
sudo timedatectl set-timezone Asia/Shanghai
sudo timedatectl set-ntp true
echo "  ✅ 时区: Asia/Shanghai"

# ---- Step 7: Swap + 内核参数 ----
echo "[7/11] 配置 swap 和内核参数..."
if [ ! -f /swapfile ]; then
    sudo fallocate -l 4G /swapfile
    sudo chmod 600 /swapfile
    sudo mkswap /swapfile
    sudo swapon /swapfile
    echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
fi
sudo sysctl -w vm.swappiness=10
sudo sysctl -w net.core.somaxconn=4096
# 持久化
sudo tee /etc/sysctl.d/99-arbitrage.conf > /dev/null <<EOF
vm.swappiness=10
net.core.somaxconn=4096
EOF
echo "  ✅ swap 4G + 内核参数已配置"

# ---- Step 8: Tailscale ----
echo "[8/11] 安装 Tailscale..."
if ! command -v tailscale &> /dev/null; then
    curl -fsSL https://tailscale.com/install.sh | sh
    sudo tailscale up --hostname="${TAILSCALE_HOSTNAME:-ali-trade}"
fi
echo "  ✅ Tailscale 已安装"

# ---- Step 9: 项目目录 ----
echo "[9/11] 创建持久化目录..."
mkdir -p ~/arbitrage-system/data/{redis,influxdb}
echo "  ✅ data/ 目录已创建"

# ---- Step 10: MOTD ----
echo "[10/11] 写入欢迎横幅..."
sudo tee /etc/motd > /dev/null <<'EOF'
=========================================
   arbitrage-system 交易服务器
   未授权访问将追究法律责任
=========================================
⚠️  风险提示：期货交易存在重大亏损风险
   本系统仅供研究学习，不构成投资建议
=========================================
EOF
echo "  ✅ MOTD 已写入"

# ---- Step 11: 重启 SSH ----
echo "[11/11] 重启 SSH 服务（新端口生效）..."
sudo systemctl restart sshd
echo "  ✅ SSH 已重启（新端口: $SSH_PORT）"

echo ""
echo "========================================="
echo " ✅ 服务器初始化完成！"
echo ""
echo " ⚠️  请重新登录验证新 SSH 端口:"
echo "    ssh -p $SSH_PORT ecs-user@$(curl -s ifconfig.me)"
echo ""
echo " 下一步:"
echo "    cd ~/arbitrage-system"
echo "    docker compose --profile full up -d"
echo "========================================="
