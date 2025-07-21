#!/bin/bash
# VoyagerSD Developer Setup Script
# Ensures a consistent environment for all contributors

set -e

echo "🚀 Starting VoyagerSD development environment setup"
echo "=================================================="

# Install required tools
echo "🔧 Installing development tools..."
make install-tools

# Generate protocol buffers code
echo "🔨 Generating gRPC code from protobuf definitions..."
make generate

# Build all binaries
echo "🛠️ Building project binaries..."
make build

# Run tests to verify setup
echo "🧪 Running test suite to verify setup..."
make test

echo ""
echo "✅ Setup completed successfully!"
echo "You're ready to develop VoyagerSD. Next steps:"
echo "1. Explore 'make help' for common commands"
echo "2. Run 'make run' to start local services"
echo "3. Check CONTRIBUTING.md for guidelines"
echo ""
echo "Happy coding! 🎉"