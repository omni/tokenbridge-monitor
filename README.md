# Tokenbridge monitor
This repo provides a real-time monitoring solution for the AMB/XDAI bridge contracts.
It performs a real-time chain indexing for the configured bridged instances, and alerts in case of some important events or outages. 

## Tech layout
* Monitor (Golang)
* PostgreSQL
* Prometheus
* Alertmanager (Slack alerts)
* Grafana (Stats visualization)
* Traefik for HTTPS routing

## Configuration
Monitor configuration is managed through the yml file, processed during the startup ([./config.yml](./config.yml)).
Config schema is described in ([./config.schema.json](./config.schema.json)).
Config supports env variable interpolation (see `INFURA_PROJECT_KEY`).

## Local start-up
1. Create env file with `INFURA_PROJECT_KEY`:
```bash
cp .env.example .env
nano .env
```
2. Build monitor docker container:
```bash
docker-compose -f docker-compose.dev.yml build
```
3. In the need of testing Slack alerts in the local deployment, do the following:
```bash
# HTTP Basic auth password for prometheus -> alertmanager authentication.
nano ./prometheus/admin_password.txt
# Slack webhook URL for sending alerts. Double check that there is no newline in the end of the file.
printf 'https://hooks.slack.com/services/...' > ./prometheus/slack_api_url.txt
```
4. Put bcrypt password hash of [./prometheus/admin_password.txt](./prometheus/admin_password.txt) in [./prometheus/web.yml](./prometheus/web.yml). Hash can be generated at https://bcrypt-generator.com:
```bash
# Set bcrypt hash for admin user
nano ./prometheus/web.yml
```
5. Start up:
```bash
# Background services
docker-compose -f docker-compose.dev.yml up -d postgres prometheus grafana
# Optionally startup alertmanager
# docker-compose -f docker-compose.dev.yml up -d alertmanager
# Startup monitor
docker-compose -f docker-compose.dev.yml up monitor
```
6. Take a look:
* http://localhost:3000 (user: admin, password: admin)
* http://localhost:9090/alerts
* http://localhost:9093/#/alerts
* http://localhost:3333/bridge/<bridge_id>
* http://localhost:3333/bridge/<bridge_id>/config
* http://localhost:3333/bridge/<bridge_id>/validators
* http://localhost:3333/chain/<chain_id>/block/<block_number>
* http://localhost:3333/chain/<chain_id>/block/<block_number>/logs
* http://localhost:3333/chain/<chain_id>/tx/<tx_hash>
* http://localhost:3333/chain/<chain_id>/tx/<tx_hash>/logs
* http://localhost:3333/tx/<tx_hash>
* http://localhost:3333/tx/<tx_hash>/logs

## Deployment
For final deployment, you will need a VM with a static IP and a DNS domain name attached to that IP.
SSL certificates will be managed by a Traefik and Let's Encrypt automatically. 
```bash
git clone https://github.com/omni/tokenbridge-monitor.git
cd tokenbridge-monitor

cp .env.example .env
nano .env # put valid INFURA_PROJECT_KEY and valid domain names
nano config.yml # modify monitor config if necessary (e.g. disable/enable particular bridges monitoring)

# HTTP Basic auth password for prometheus -> alertmanager authentication.
nano ./prometheus/admin_password.txt
# Slack webhook URL for sending alerts. Double check that there is no newline in the end of the file.
printf 'https://hooks.slack.com/services/...' > ./prometheus/slack_api_url.txt
# Set bcrypt hash for admin user
nano ./prometheus/web.yml

docker-compose -f docker-compose.prod.yml pull
docker-compose -f docker-compose.prod.yml up -d
```
After a small delay you should be able to access all the services via the provided DNS names through HTTPS.
Make sure to change the default admin password in Grafana at the first login.
