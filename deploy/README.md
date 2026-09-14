# Voice2Canvas Deployment Examples

These files provide reference configurations and helper scripts for self-hosting Voice2Canvas on a single Linux host. They are examples only; none point at a real production deployment or hosted service.

- `install.sh`: Installs and enables systemd user service units (`voice2canvas-backend.service` and `voice2canvas-frontend.service`) for the login user.
- `verify-production.sh`: Health check script verifying HTTP `/healthz` and optionally checking `voice2canvas.service` status.
- `voice2canvas.service`: Example hardened systemd system service unit for running the standalone server binary under a dedicated user.
- `voice2canvas-backend.service`: Example systemd user service unit for running the Go backend server.
- `voice2canvas-frontend.service`: Example systemd user service unit for running the Vite development server on LAN.
- `voice2canvas.env.example`: Example environment configuration template for server ports, Gemini API credentials, models, and logging.
