#!/bin/bash

set -e

REPO=https://github.com/__REPOSITORY__/releases/download
RELEASE=__RELEASE__

GENESIS="${REPO}/${RELEASE}/genesis.zip"
BINARY_AMD64="${REPO}/${RELEASE}/geth-${RELEASE}-linux-amd64.zip"
BINARY_ARM64="${REPO}/${RELEASE}/geth-${RELEASE}-linux-arm64.zip"

ask() {
    MSG="$1"
    VAR="$2"
    DEFAULT="$3"
    OPTS="$4"

    if [ -n "${!VAR}" ]; then
        return
    fi

    msg="$MSG"
    if [ -n "$DEFAULT" ]; then
        msg="$msg (default = $DEFAULT)"
    fi
    msg="${msg}: "

    read $OPTS -p "$msg" "$VAR"
    if [ -z "${!VAR}" ]; then
        eval "$VAR"="$DEFAULT"
    fi
}

msg_err() {
    printf '\033[31m\x1b[1m%s\x1b[1m\033[m\n' "Error: $1" 1>&2
}

msg_blue() {
    printf '\033[34m\x1b[1m%s\x1b[1m\033[m\n' "$1"
}

msg_green() {
    printf '\033[32m\x1b[1m%s\x1b[1m\033[m\n' "$1"
}

spacer() {
    echo -e "\n"
}

# Check if systemd can be used. 
if ! systemctl --version >/dev/null 2>&1; then
    msg_err "Unsupported operating system."
    exit 1
fi

# Check cpu architecture.
case "$(uname -p)" in
    "x86_64")
        BINARY=$BINARY_AMD64
        ;;
    "arm64"|"aarch64")
        BINARY=$BINARY_ARM64
        ;;
    *)
        msg_err "Unsupported processor architecture."
        exit 1
esac

# Check if wget or curl can be used.
if wget -h >/dev/null 2>&1; then
    download() {
        wget "$1" -O "$2"
    }
elif curl -h >/dev/null 2>&1; then
    download() {
        curl "$1" -o "$2" -L --fail
    }
else
    msg_err "Please install the wget or curl command."
    exit 1
fi

# Check if unzip can be used.
if ! unzip -h >/dev/null 2>&1; then
    msg_err "Please install the unzip command." 
    exit 1
fi

# Ask for parameters.
ask "Select a network [1: mainnet, 2: testnet]" NETWORK 1
ask "Enter the binary installation path" INSTALL_PATH "/usr/local/bin/geth"
ask "Enter the os user name of systemd service" SERVICE_USER "geth"
ask "Enter the passphrase for the private key" PASSPHRASE "" "-s"
ask $'\nDo you want to start block validation automatically? (WARNING: Save the passphrase to disk.)' SAVE_PASSPHRASE "no" "-e"

if [ -d "$INSTALL_PATH" ]; then
    msg_err "$INSTALL_PATH is a directory."
    exit 1
fi

install_dir="$(dirname "$INSTALL_PATH")"
if [ "$install_dir" == "." ] || [ ! -d "$install_dir" ]; then
    msg_err "Install directory does not exist."
    exit 1
fi

if [ -z "$PASSPHRASE" ]; then
    msg_err "Please enter the passphrase for the private key."
    exit 1
fi

case "$NETWORK" in
    1)
        NETWORK=mainnet
        NETWORK_ID=248
        ENODE="enode://093c363d9fa759b58cb0a59d8ca664b4b4981873dc0305b113edf6d0c865089ed9894300b385e58bb3da2f7b8b575170522c5f542a9d47cbff7d28d3c8c8dd65@35.73.174.118:30303"
        ;;
    2)
        NETWORK=testnet
        NETWORK_ID=9372
        ENODE="enode://4a85df39ec500acd31d4b9feeea1d024afee5e8df4bc29325c2abf2e0a02a34f6ece24aca06cb5027675c167ecf95a9fc23fb7a0f671f84edb07dafe6e729856@35.77.156.6:30303"
        ;;
    *)
        msg_err "Select 1 or 2 for the network."
        exit 1
esac

HOME_DIR=/home/$SERVICE_USER
WALLET_FILE=$HOME_DIR/.ethereum/wallet.txt
PASSWORD_FILE=$HOME_DIR/.ethereum/password.txt
STATIC_NODE_FILE=$HOME_DIR/.ethereum/geth/static-nodes.json
UNIT_FILE=/usr/lib/systemd/system/geth.service

cd $(mktemp -d)

spacer

msg_blue "1. Create a user"
if id $SERVICE_USER >/dev/null 2>&1; then
    echo skip
else
    useradd -s /sbin/nologin $SERVICE_USER
    if [ ! -d $HOME_DIR ]; then
        mkdir $HOME_DIR
        chown $SERVICE_USER:$SERVICE_USER $HOME_DIR
        chmod 700 $HOME_DIR
    fi
    echo "Created: $(id -a $SERVICE_USER)"
fi
chmod o+rx .

spacer

msg_blue "2. Download the binary from GitHub"
if [ -x $INSTALL_PATH ]; then
    echo skip
else
    download $BINARY oasys-geth.zip
    unzip oasys-geth.zip
    mv geth $INSTALL_PATH
fi

spacer

msg_blue "3. Create a genesis block"
if [ -d $HOME_DIR/.ethereum/geth ]; then
    echo skip
else
    download $GENESIS oasys-genesis.zip
    unzip oasys-genesis.zip
    sudo -u $SERVICE_USER $INSTALL_PATH init genesis/${NETWORK}.json
fi

spacer

msg_blue "4. Create a private key"
if [ -f $WALLET_FILE ]; then
    echo skip
else
    echo -n "$PASSPHRASE" > $PASSWORD_FILE
    sudo -u $SERVICE_USER $INSTALL_PATH account new --password $PASSWORD_FILE > $WALLET_FILE
fi
ETHERBASE=$(grep "Public address of the key" $WALLET_FILE | sed -e "s#.*\(0x.*\)#\1#g")
if [ "$SAVE_PASSPHRASE" == yes ]; then
    SYSTEMD_OPTS=" \\\\\n  --mine --unlock \${ETHERBASE} --password \${PASSWORD} --allow-insecure-unlock"
else
    rm -f $PASSWORD_FILE
fi

spacer

msg_blue "5. Create a static node configuration"
if [ -f $STATIC_NODE_FILE ]; then
    echo skip
else
    echo -n "[\"${ENODE}\"]" > $STATIC_NODE_FILE
    echo "Created: $STATIC_NODE_FILE"
fi

spacer

msg_blue  "6. Create a systemd unit"
if [ -f $UNIT_FILE ]; then
    echo skip
else
echo -e "[Unit]
Description=Validator for Oasys Blockchain.
After=network.target

[Service]
User=$SERVICE_USER
Type=simple

Environment=DATA_DIR=$HOME_DIR/.ethereum
Environment=NETWORK_ID=$NETWORK_ID
Environment=ETHERBASE=$ETHERBASE
Environment=PASSWORD=$PASSWORD_FILE
Environment=GASLIMIT=30000000

ExecStart=$INSTALL_PATH \\
  --datadir \${DATA_DIR} \\
  --networkid \${NETWORK_ID} \\
  --miner.etherbase \${ETHERBASE} --miner.gaslimit \${GASLIMIT}$SYSTEMD_OPTS \\
  --syncmode full --gcmode archive

KillMode=process
KillSignal=SIGINT
TimeoutStopSec=90

Restart=on-failure
RestartSec=30s

[Install]
WantedBy=multi-user.target" > $UNIT_FILE
echo "Created: $UNIT_FILE"
systemctl daemon-reload
fi

spacer

msg_green "Setup successful."
