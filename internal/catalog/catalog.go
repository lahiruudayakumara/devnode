package catalog

import "sort"

type Service struct {
	Name, Label, Method, Description, Script, Manual string
}

var services = map[string]Service{
	"docker":        {Name: "docker", Label: "Docker Engine + Compose", Method: "Ubuntu packages", Description: "Container runtime required by container services.", Script: `sudo apt-get update && sudo apt-get install -y docker.io docker-compose-v2 && sudo systemctl enable --now docker && sudo usermod -aG docker "$USER"; printf '%s\n' 'Docker installed. New SSH sessions will receive Docker group access.'`},
	"postgresql":    {Name: "postgresql", Label: "PostgreSQL", Method: "Ubuntu package", Description: "Relational development database; Ubuntu defaults keep it local.", Script: `sudo apt-get update && sudo apt-get install -y postgresql && sudo systemctl enable --now postgresql`},
	"redis":         {Name: "redis", Label: "Redis", Method: "Ubuntu package", Description: "Cache and message broker.", Script: `sudo apt-get update && sudo apt-get install -y redis-server && sudo systemctl enable --now redis-server`},
	"mosquitto":     {Name: "mosquitto", Label: "Mosquitto MQTT", Method: "Ubuntu package", Description: "MQTT broker for ESP32 and IoT development.", Script: `sudo apt-get update && sudo apt-get install -y mosquitto mosquitto-clients && sudo systemctl enable --now mosquitto`},
	"portainer":     {Name: "portainer", Label: "Portainer CE", Method: "Docker container", Description: "Docker web administration on localhost:9443.", Script: dockerRun(`volume create portainer_data >/dev/null; d run -d --name portainer --restart=always -p 127.0.0.1:9443:9443 -v /var/run/docker.sock:/var/run/docker.sock -v portainer_data:/data portainer/portainer-ce:sts`)},
	"uptime-kuma":   {Name: "uptime-kuma", Label: "Uptime Kuma", Method: "Docker container", Description: "Uptime monitoring on localhost:3001.", Script: dockerRun(`volume create uptime-kuma >/dev/null; d run -d --name uptime-kuma --restart=unless-stopped -p 127.0.0.1:3001:3001 -v uptime-kuma:/app/data louislam/uptime-kuma:2`)},
	"dozzle":        {Name: "dozzle", Label: "Dozzle", Method: "Docker container", Description: "Read-only Docker log viewer on localhost:8080.", Script: dockerRun(`volume create dozzle_data >/dev/null; d run -d --name dozzle --restart=unless-stopped -p 127.0.0.1:8080:8080 -v /var/run/docker.sock:/var/run/docker.sock:ro -v dozzle_data:/data amir20/dozzle:latest`)},
	"mailpit":       {Name: "mailpit", Label: "Mailpit", Method: "Docker container", Description: "Test SMTP on localhost:1025 and UI on localhost:8025.", Script: dockerRun(`run -d --name mailpit --restart=unless-stopped -p 127.0.0.1:8025:8025 -p 127.0.0.1:1025:1025 axllent/mailpit:latest`)},
	"node-red":      {Name: "node-red", Label: "Node-RED", Method: "Docker container", Description: "IoT workflow editor on localhost:1880.", Script: dockerRun(`volume create node-red-data >/dev/null; d run -d --name node-red --restart=unless-stopped -p 127.0.0.1:1880:1880 -v node-red-data:/data nodered/node-red:latest`)},
	"mongodb":       {Name: "mongodb", Label: "MongoDB", Method: "Docker container", Description: "Development document database on localhost:27017.", Script: dockerRun(`volume create mongodb-data >/dev/null; d run -d --name mongodb --restart=unless-stopped -p 127.0.0.1:27017:27017 -v mongodb-data:/data/db mongo:8`)},
	"localstack":    {Name: "localstack", Label: "LocalStack", Method: "guided", Description: "Local AWS emulation.", Manual: "LocalStack now requires an account/auth-token workflow. Configure the token first, then follow https://docs.localstack.cloud/aws/getting-started/installation/."},
	"minio":         {Name: "minio", Label: "MinIO", Method: "guided", Description: "S3-compatible object storage.", Manual: "MinIO requires choosing and securely storing administrator credentials. Use the official deployment guide rather than an auto-generated password."},
	"netdata":       {Name: "netdata", Label: "Netdata", Method: "guided", Description: "Real-time host monitoring.", Manual: "Netdata's official installer downloads and executes a changing vendor script; DevNode does not execute remote scripts automatically."},
	"github-runner": {Name: "github-runner", Label: "GitHub Actions Runner", Method: "guided", Description: "Self-hosted CI runner.", Manual: "Create the runner in GitHub first and use its short-lived registration token. Run only trusted workflows on this host."},
	"ros2":          {Name: "ros2", Label: "ROS 2", Method: "guided", Description: "Robotics middleware.", Manual: "ROS 2 installation is distribution-specific. Confirm that your ROS release supports this Ubuntu version before adding its apt repository."},
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
