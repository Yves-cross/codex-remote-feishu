#!/bin/bash
set -e

cd /data/dl/fschannel

# Initialize git if needed
if [ ! -d .git ]; then
  git init
fi

# Create .env from template if not exists
if [ ! -f .env ]; then
  cat > .env << 'EOF'
# Relay Server
RELAY_PORT=9500
RELAY_API_PORT=9501
SESSION_GRACE_PERIOD=300
MESSAGE_BUFFER_SIZE=100

# Feishu Bot
FEISHU_APP_ID=
FEISHU_APP_SECRET=
RELAY_API_URL=http://localhost:9501

# Wrapper (also configurable via CLI args)
RELAY_SERVER_URL=ws://localhost:9500
CODEX_REAL_BINARY=codex
EOF
  echo "Created .env template — fill in FEISHU_APP_ID and FEISHU_APP_SECRET"
fi

# Ensure .gitignore excludes sensitive files
if [ ! -f .gitignore ]; then
  cat > .gitignore << 'EOF'
node_modules/
dist/
target/
.env
*.log
EOF
fi

# Install dependencies if package.json exists
if [ -f package.json ]; then
  npm install
fi

if [ -f shared/package.json ]; then
  cd shared && npm install && cd ..
fi

if [ -f server/package.json ]; then
  cd server && npm install && cd ..
fi

if [ -f bot/package.json ]; then
  cd bot && npm install && cd ..
fi

# Build Rust wrapper if Cargo.toml exists
if [ -f wrapper/Cargo.toml ]; then
  cd wrapper && cargo build && cd ..
fi

echo "Init complete."
