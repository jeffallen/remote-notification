.PHONY: all build test install clean uninstall android help

# Default target
all: build test

# Build Go servers
build:
	@echo "Building Go servers..."
	cd app-backend && go build -o ../bin/app-backend .
	cd notification-backend && go build -o ../bin/notification-backend .
	@echo "Build complete. Binaries in ./bin/"

# Run tests
test:
	@echo "Running Go tests..."
	cd app-backend && go test -v ./...
	cd notification-backend && go test -v ./...
	@echo "All tests passed"

# Build Android app
android:
	@echo "Building Android app..."
	cd android-fcm-app && ./gradlew build
	@echo "Android build complete"

# Install Go servers to /usr/bin (requires sudo)
install: build
	@echo "Installing Go servers..."
	# Create users and directories
	sudo useradd -r -s /bin/false app-backend || true
	sudo useradd -r -s /bin/false notification-backend || true
	sudo mkdir -p /var/lib/app-backend
	sudo mkdir -p /var/lib/notification-backend
	sudo chown app-backend:app-backend /var/lib/app-backend
	sudo chown notification-backend:notification-backend /var/lib/notification-backend
	# Install binaries
	sudo cp bin/app-backend /usr/bin/app-backend
	sudo cp bin/notification-backend /usr/bin/notification-backend
	sudo chmod +x /usr/bin/app-backend
	sudo chmod +x /usr/bin/notification-backend
	# Install systemd services
	sudo cp systemd/app-backend.service /etc/systemd/system/
	sudo cp systemd/notification-backend.service /etc/systemd/system/
	sudo systemctl daemon-reload
	@echo "Installation complete. Enable services with:"
	@echo "  sudo systemctl enable app-backend"
	@echo "  sudo systemctl enable notification-backend"
	@echo "  sudo systemctl start app-backend"
	@echo "  sudo systemctl start notification-backend"

# Uninstall Go servers
uninstall:
	@echo "Uninstalling Go servers..."
	# Stop and disable services
	sudo systemctl stop app-backend || true
	sudo systemctl stop notification-backend || true
	sudo systemctl disable app-backend || true
	sudo systemctl disable notification-backend || true
	# Remove systemd services
	sudo rm -f /etc/systemd/system/app-backend.service
	sudo rm -f /etc/systemd/system/notification-backend.service
	sudo systemctl daemon-reload
	# Remove binaries
	sudo rm -f /usr/bin/app-backend
	sudo rm -f /usr/bin/notification-backend
	# Remove data directories (optional, commented out for safety)
	# sudo rm -rf /var/lib/app-backend
	# sudo rm -rf /var/lib/notification-backend
	@echo "Uninstallation complete"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	cd app-backend && go clean
	cd notification-backend && go clean
	cd android-fcm-app && ./gradlew clean
	@echo "Clean complete"

# Create bin directory
bin:
	mkdir -p bin

# Override build to depend on bin directory
build: bin

help:
	@echo "Available targets:"
	@echo "  all        - Build and test Go servers (default)"
	@echo "  build      - Build Go servers"
	@echo "  test       - Run Go tests"
	@echo "  android    - Build Android app"
	@echo "  install    - Install Go servers to /usr/bin (requires sudo)"
	@echo "  uninstall  - Uninstall Go servers (requires sudo)"
	@echo "  clean      - Clean build artifacts"
	@echo "  help       - Show this help message"
