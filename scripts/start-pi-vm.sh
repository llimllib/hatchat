#!/bin/bash
# Script to start a Lima VM with pi-coding-agent installed
# Mounts current directory and ~/.pi for config/auth

set -e

VM_NAME="pi-agent"
CURRENT_DIR="$(pwd)"
PI_CONFIG_DIR="$HOME/.pi"

# Create ~/.pi if it doesn't exist
mkdir -p "$PI_CONFIG_DIR"

# Check if VM already exists
if limactl list -q | grep -q "^${VM_NAME}$"; then
    echo "VM '$VM_NAME' already exists."
    
    # Check if it's running
    if limactl list --format json | jq -e ".[] | select(.name == \"$VM_NAME\" and .status == \"Running\")" > /dev/null 2>&1; then
        echo "VM is already running."
    else
        echo "Starting existing VM..."
        limactl start "$VM_NAME"
    fi
else
    echo "Creating new Lima VM '$VM_NAME'..."
    
    # Create a temporary Lima config file
    LIMA_CONFIG=$(mktemp)
    cat > "$LIMA_CONFIG" << EOF
# Lima VM configuration for pi-coding-agent

# Use Ubuntu 25.04 (Plucky Puffin)
images:
  - location: "https://cloud-images.ubuntu.com/releases/25.04/release/ubuntu-25.04-server-cloudimg-amd64.img"
    arch: "x86_64"
  - location: "https://cloud-images.ubuntu.com/releases/25.04/release/ubuntu-25.04-server-cloudimg-arm64.img"
    arch: "aarch64"

# Mount current working directory
mounts:
  - location: "$CURRENT_DIR"
    writable: true
  - location: "$PI_CONFIG_DIR"
    writable: true
    mountPoint: "/home/\${LIMA_CIDATA_USER}.linux/.pi"

# Provision script to install Node.js and pi-coding-agent
provision:
  - mode: system
    script: |
      #!/bin/bash
      set -eux -o pipefail
      
      # Install Node.js 24.x
      curl -fsSL https://deb.nodesource.com/setup_24.x | bash -
      apt-get install -y nodejs
      
      # Install pi-coding-agent globally
      npm install -g @mariozechner/pi-coding-agent
      
      echo "pi-coding-agent installed successfully!"

# CPU and memory allocation
cpus: 4
memory: "8GiB"
disk: "20GiB"

# Enable SSH agent forwarding (useful for git operations)
ssh:
  forwardAgent: true
EOF

    # Create and start the VM
    limactl create --name "$VM_NAME" "$LIMA_CONFIG"
    rm "$LIMA_CONFIG"
    
    limactl start "$VM_NAME"
fi

echo ""
echo "============================================"
echo "Lima VM '$VM_NAME' is ready!"
echo "============================================"
echo ""
echo "Mounted directories:"
echo "  - Current dir: $CURRENT_DIR"
echo "  - Pi config:   $PI_CONFIG_DIR -> ~/.pi (in VM)"
echo ""
echo "Usage:"
echo "  # Enter the VM shell:"
echo "  limactl shell $VM_NAME"
echo ""
echo "  # Run pi directly:"
echo "  limactl shell $VM_NAME -- pi"
echo ""
echo "  # Run pi in the mounted current directory:"
echo "  limactl shell $VM_NAME --workdir \"$CURRENT_DIR\" -- pi"
echo ""
echo "  # Stop the VM:"
echo "  limactl stop $VM_NAME"
echo ""
echo "  # Delete the VM:"
echo "  limactl delete $VM_NAME"
echo ""
