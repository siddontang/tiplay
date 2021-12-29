# tiplay

A toy project to play music (aha, mostly Noise in fact) through Prometheus metrics.

This project is inspired by [prometheus_video_renderer](https://github.com/MacroPower/prometheus_video_renderer). 

## Notice

- I haven't tuned a good way for the audio sample, so maybe you won't hear any sound. I run the benchmark for a long time(1h+), and can hear the noise.
- Don't test this in the office, you may disturb your colleagues.

## Usage

### Build 

```bash
go build -o tiplay main.go
```

### Start a Prometheus

Here we use [`tiup`](https://github.com/pingcap/tiup) to start a TiDB cluster in the machine.

```bash
# Install tiup
curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh

# Start a TiDB playground, this will also start a Prometheus server
# Here we start a TiDB cluseter with 5.3 version 
tiup --tag 5.3 playground 5.3
```

The output may look like below:

```bash
Playground Bootstrapping...
Start pd instance
Start tikv instance
Start tidb instance
Waiting for tidb instances ready
127.0.0.1:4000 ... Done
Start tiflash instance
Waiting for tiflash instances ready
127.0.0.1:3930 ... Done
CLUSTER START SUCCESSFULLY, Enjoy it ^-^
To connect TiDB: mysql --comments --host 127.0.0.1 --port 4000 -u root -p (no password)
To view the dashboard: http://127.0.0.1:2379/dashboard
PD client endpoints: [127.0.0.1:2379]
To view the Prometheus: http://127.0.0.1:50344
To view the Grafana: http://127.0.0.1:3000
```

### Run a benchmark to the TiDB, so we can query metrics from Prometheus later

Here we run a TPCC benchmark with [go-tpc](https://github.com/pingcap/go-tpc). We can already use this tool with `tiup` directly.

```bash
# Prepare the data
tiup bench tpcc --warehouses 4 prepare

# Run the benchmark
tiup bench tpcc --warehouses 4 run
```

### Long time later...

```bash
tiplay -prom_url http://127.0.0.1:50344 -track "sum(rate(tidb_executor_statement_total[1m])) by (type)"
```

Have a fun and please listen to the Noise!!!


