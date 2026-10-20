package catalog

import "sort"

// Service describes one installable service in the catalog.
// Exactly one of Script or Manual must be non-empty.
//   - Script  → DevNode runs it automatically after confirmation.
//   - Manual  → human-guided; Steps holds the numbered commands to print.
type Service struct {
	Name, Label, Method, Description string
	Script                           string   // shell script for auto-install
	Manual                           string   // one-line reason why guided is needed
	Steps                            []string // ordered commands shown to the user
}

var services = map[string]Service{
	"docker": {
		Name: "docker", Label: "Docker Engine + Compose",
		Method:      "Ubuntu packages",
		Description: "Container runtime required by container services.",
		Script:      `sudo apt-get update && sudo apt-get install -y docker.io docker-compose-v2 && sudo systemctl enable --now docker && sudo usermod -aG docker "$USER"; printf '%s\n' 'Docker installed. New SSH sessions will receive Docker group access.'`,
	},
	"postgresql": {
		Name: "postgresql", Label: "PostgreSQL",
		Method:      "Ubuntu package",
		Description: "Relational development database; Ubuntu defaults keep it local.",
		Script:      `sudo apt-get update && sudo apt-get install -y postgresql && sudo systemctl enable --now postgresql`,
	},
	"redis": {
		Name: "redis", Label: "Redis",
		Method:      "Ubuntu package",
		Description: "Cache and message broker.",
		Script:      `sudo apt-get update && sudo apt-get install -y redis-server && sudo systemctl enable --now redis-server`,
	},
	"mosquitto": {
		Name: "mosquitto", Label: "Mosquitto MQTT",
		Method:      "Ubuntu package",
		Description: "MQTT broker for ESP32 and IoT development.",
		Script:      `sudo apt-get update && sudo apt-get install -y mosquitto mosquitto-clients && sudo systemctl enable --now mosquitto`,
	},
	"portainer": {
		Name: "portainer", Label: "Portainer CE",
		Method:      "Docker container",
		Description: "Docker web administration on localhost:9443.",
		Script:      dockerRun(`volume create portainer_data >/dev/null; d run -d --name portainer --restart=always -p 127.0.0.1:9443:9443 -v /var/run/docker.sock:/var/run/docker.sock -v portainer_data:/data portainer/portainer-ce:sts`),
	},
	"uptime-kuma": {
		Name: "uptime-kuma", Label: "Uptime Kuma",
		Method:      "Docker container",
		Description: "Uptime monitoring on localhost:3001.",
		Script:      dockerRun(`volume create uptime-kuma >/dev/null; d run -d --name uptime-kuma --restart=unless-stopped -p 127.0.0.1:3001:3001 -v uptime-kuma:/app/data louislam/uptime-kuma:2`),
	},
	"dozzle": {
		Name: "dozzle", Label: "Dozzle",
		Method:      "Docker container",
		Description: "Read-only Docker log viewer on localhost:8080.",
		Script:      dockerRun(`volume create dozzle_data >/dev/null; d run -d --name dozzle --restart=unless-stopped -p 127.0.0.1:8080:8080 -v /var/run/docker.sock:/var/run/docker.sock:ro -v dozzle_data:/data amir20/dozzle:latest`),
	},
	"mailpit": {
		Name: "mailpit", Label: "Mailpit",
		Method:      "Docker container",
		Description: "Test SMTP on localhost:1025 and UI on localhost:8025.",
		Script:      dockerRun(`run -d --name mailpit --restart=unless-stopped -p 127.0.0.1:8025:8025 -p 127.0.0.1:1025:1025 axllent/mailpit:latest`),
	},
	"node-red": {
		Name: "node-red", Label: "Node-RED",
		Method:      "Docker container",
		Description: "IoT workflow editor on localhost:1880.",
		Script:      dockerRun(`volume create node-red-data >/dev/null; d run -d --name node-red --restart=unless-stopped -p 127.0.0.1:1880:1880 -v node-red-data:/data nodered/node-red:latest`),
	},
	"mongodb": {
		Name: "mongodb", Label: "MongoDB",
		Method:      "Docker container",
		Description: "Development document database on localhost:27017.",
		Script: `if ! command -v docker >/dev/null 2>&1; then printf '%s\n' 'Docker is required. Install docker first.' >&2; exit 3; fi
d(){ if docker info >/dev/null 2>&1; then docker "$@"; else sudo docker "$@"; fi; }

# ── Port conflict check ───────────────────────────────────────────────────────
if ss -ltn 2>/dev/null | grep -q '127\.0\.0\.1:27017\|0\.0\.0\.0:27017\|\*:27017'; then
  printf 'ERROR: port 27017 is already in use on this host.\n' >&2
  printf 'Running containers that may hold it:\n' >&2
  docker ps --format '  {{.Names}}  {{.Image}}  {{.Ports}}' 2>/dev/null | grep 27017 || true
  printf 'Resolve the conflict first, then re-run: devnode install mongodb\n' >&2
  exit 3
fi

# ── CPU compatibility check (MongoDB 5+ requires AVX) ────────────────────────
if grep -qw avx /proc/cpuinfo 2>/dev/null; then
  mongo_image=mongo:8
  printf 'AVX detected — using MongoDB 8\n'
else
  mongo_image=mongo:4.4
  printf 'No AVX detected — using MongoDB 4.4 (compatible with this CPU)\n'
fi

# ── Pull and start ────────────────────────────────────────────────────────────
d volume create mongodb-data >/dev/null
d run -d --name mongodb --restart=unless-stopped \
  -p 127.0.0.1:27017:27017 \
  -v mongodb-data:/data/db \
  "$mongo_image"
printf 'MongoDB started (%s). Connect with: devnode tunnel mongodb\n' "$mongo_image"`,
	},
	"jupyter": {
		Name: "jupyter", Label: "Jupyter Notebook",
		Method:      "Docker container",
		Description: "Interactive notebooks on localhost:8888.",
		Script:      dockerRun(`volume create jupyter-data >/dev/null; d run -d --name jupyter --restart=unless-stopped -p 127.0.0.1:8888:8888 -v jupyter-data:/home/jovyan/work -e JUPYTER_ENABLE_LAB=yes jupyter/minimal-notebook:latest start-notebook.py --NotebookApp.token='' --NotebookApp.password='' --ip=0.0.0.0`),
	},

	// ── Guided installs ───────────────────────────────────────────────────────

	"ros2": {
		Name: "ros2", Label: "ROS 2",
		Method:      "guided",
		Description: "Robotics middleware.",
		Manual:      "ROS 2 is distribution-specific; the apt repository must match your Ubuntu release.",
		Steps: []string{
			`# 1. Check your Ubuntu version`,
			`lsb_release -a`,
			`#    Ubuntu 24.04 → Jazzy Jalisco  |  Ubuntu 22.04 → Humble Hawksbill`,
			``,
			`# 2. Set locale`,
			`sudo apt-get install -y locales && sudo locale-gen en_US en_US.UTF-8 && sudo update-locale LC_ALL=en_US.UTF-8 LANG=en_US.UTF-8`,
			``,
			`# 3. Add the ROS 2 apt repository (Jazzy / Ubuntu 24.04 example)`,
			`sudo apt-get install -y software-properties-common curl`,
			`sudo curl -sSL https://raw.githubusercontent.com/ros/rosdistro/master/ros.key -o /usr/share/keyrings/ros-archive-keyring.gpg`,
			`echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/ros-archive-keyring.gpg] http://packages.ros.org/ros2/ubuntu $(. /etc/os-release && echo $UBUNTU_CODENAME) main" | sudo tee /etc/apt/sources.list.d/ros2.list`,
			``,
			`# 4. Install ROS 2 (desktop for full tools, base for headless)`,
			`sudo apt-get update && sudo apt-get install -y ros-jazzy-ros-base`,
			``,
			`# 5. Source ROS in every SSH session`,
			`echo "source /opt/ros/jazzy/setup.bash" >> ~/.bashrc`,
			``,
			`# 6. Verify`,
			`source /opt/ros/jazzy/setup.bash && ros2 --version`,
		},
	},

	"minio": {
		Name: "minio", Label: "MinIO",
		Method:      "guided",
		Description: "S3-compatible object storage.",
		Manual:      "MinIO requires you to choose and securely store administrator credentials before install.",
		Steps: []string{
			`# 1. Create a dedicated data directory`,
			`sudo mkdir -p /data/minio`,
			``,
			`# 2. Download the MinIO binary`,
			`curl -sSL https://dl.min.io/server/minio/release/linux-amd64/minio -o /tmp/minio`,
			`sudo install /tmp/minio /usr/local/bin/minio`,
			``,
			`# 3. Create a systemd env file with YOUR own credentials`,
			`#    Replace CHANGE_ME with strong values before running`,
			`sudo tee /etc/default/minio <<'EOF'
MINIO_ROOT_USER=CHANGE_ME
MINIO_ROOT_PASSWORD=CHANGE_ME_STRONG_PASSWORD
MINIO_VOLUMES=/data/minio
MINIO_OPTS="--address 127.0.0.1:9000 --console-address 127.0.0.1:9001"
EOF`,
			``,
			`# 4. Create systemd unit`,
			`sudo tee /etc/systemd/system/minio.service <<'EOF'
[Unit]
Description=MinIO Object Storage
After=network.target

[Service]
EnvironmentFile=/etc/default/minio
ExecStart=/usr/local/bin/minio server $MINIO_VOLUMES $MINIO_OPTS
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF`,
			``,
			`# 5. Enable and start`,
			`sudo systemctl daemon-reload && sudo systemctl enable --now minio`,
			``,
			`# 6. Open the console (via devnode tunnel minio)`,
			`devnode tunnel minio`,
		},
	},

	"netdata": {
		Name: "netdata", Label: "Netdata",
		Method:      "guided",
		Description: "Real-time host monitoring.",
		Manual:      "Netdata's official installer is a vendor script; review it before execution.",
		Steps: []string{
			`# 1. Review the installer first (recommended)`,
			`curl -sSL https://get.netdata.cloud/kickstart.sh | less`,
			``,
			`# 2. Run the installer (native, no Docker required)`,
			`curl -sSL https://get.netdata.cloud/kickstart.sh | sudo bash`,
			``,
			`# 3. Confirm the service is running`,
			`sudo systemctl status netdata --no-pager`,
			``,
			`# 4. Forward the dashboard to your Mac (port 19999)`,
			`devnode tunnel netdata`,
		},
	},

	"localstack": {
		Name: "localstack", Label: "LocalStack",
		Method:      "guided",
		Description: "Local AWS emulation.",
		Manual:      "LocalStack now requires an account and auth-token before install.",
		Steps: []string{
			`# 1. Sign up / log in and copy your auth token`,
			`#    https://app.localstack.cloud/sign-in`,
			``,
			`# 2. Set the token in your shell environment`,
			`#    Replace <YOUR_TOKEN> with the value from the dashboard`,
			`export LOCALSTACK_AUTH_TOKEN=<YOUR_TOKEN>`,
			``,
			`# 3. Run LocalStack via Docker`,
			`docker run -d --name localstack --restart=unless-stopped \`,
			`  -p 127.0.0.1:4566:4566 \`,
			`  -v /var/run/docker.sock:/var/run/docker.sock \`,
			`  -e LOCALSTACK_AUTH_TOKEN \`,
			`  localstack/localstack`,
			``,
			`# 4. Test with the AWS CLI`,
			`aws --endpoint-url=http://localhost:4566 s3 ls`,
		},
	},

	"github-runner": {
		Name: "github-runner", Label: "GitHub Actions Runner",
		Method:      "guided",
		Description: "Self-hosted CI runner.",
		Manual:      "A short-lived registration token from GitHub is required before install.",
		Steps: []string{
			`# 1. On GitHub: Settings → Actions → Runners → New self-hosted runner`,
			`#    Copy the short-lived token shown on that page`,
			``,
			`# 2. Create runner directory`,
			`mkdir -p ~/actions-runner && cd ~/actions-runner`,
			``,
			`# 3. Download the latest runner (x64 Linux)`,
			`curl -sSL https://github.com/actions/runner/releases/latest/download/actions-runner-linux-x64.tar.gz | tar xz`,
			``,
			`# 4. Configure — replace ORG/REPO and TOKEN`,
			`./config.sh --url https://github.com/ORG/REPO --token TOKEN --unattended --name "$(hostname)"`,
			``,
			`# 5. Install and start as a systemd service`,
			`sudo ./svc.sh install && sudo ./svc.sh start`,
			``,
			`# 6. Verify`,
			`sudo ./svc.sh status`,
		},
	},
}

func dockerRun(body string) string {
	return `if ! command -v docker >/dev/null 2>&1; then printf '%s\n' 'Docker is required. Install docker first.' >&2; exit 3; fi; d(){ if docker info >/dev/null 2>&1; then docker "$@"; else sudo docker "$@"; fi; }; d ` + body
}

func Get(name string) (Service, bool) { s, ok := services[name]; return s, ok }

func Names() []string {
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
