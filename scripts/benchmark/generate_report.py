#!/usr/bin/env python3
"""Generate an HTML benchmark comparison report from results.json."""

import json
import os
import sys
from datetime import datetime

TEMPLATE = """<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Feather Benchmark Results</title>
<style>
  body {{ font-family: -apple-system, sans-serif; max-width: 900px; margin: 0 auto; padding: 40px 20px; background: #0d1117; color: #e6edf3; }}
  h1 {{ border-bottom: 1px solid #30363d; padding-bottom: 12px; }}
  h1 span {{ color: #58a6ff; }}
  .meta {{ color: #8b949e; font-size: 14px; margin-bottom: 24px; }}
  table {{ width: 100%; border-collapse: collapse; margin: 20px 0; }}
  th {{ background: #161b22; text-align: left; padding: 10px 16px; border: 1px solid #30363d; font-size: 13px; }}
  td {{ padding: 10px 16px; border: 1px solid #30363d; font-size: 14px; }}
  .good {{ color: #3fb950; font-weight: 600; }}
  .bar {{ background: #30363d; border-radius: 4px; height: 20px; position: relative; }}
  .bar-fill {{ background: #58a6ff; border-radius: 4px; height: 100%; }}
  .note {{ background: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 16px; margin: 20px 0; font-size: 13px; color: #8b949e; }}
  footer {{ margin-top: 40px; padding-top: 20px; border-top: 1px solid #30363d; font-size: 12px; color: #8b949e; }}
</style>
</head>
<body>
<h1>🪶 <span>Feather</span> Benchmark Results</h1>
<p class="meta">Generated: {timestamp} | Entities: {entities:,} | Features: {features} | Concurrency: {concurrency}</p>

<h2>Latency (microseconds, lower is better)</h2>
<table>
<tr><th>Metric</th><th>Feather</th><th>Visualization</th></tr>
<tr><td>Point Lookup P50</td><td class="good">{p50} µs</td><td><div class="bar"><div class="bar-fill" style="width: {p50_pct}%"></div></div></td></tr>
<tr><td>Point Lookup P99</td><td class="good">{p99} µs</td><td><div class="bar"><div class="bar-fill" style="width: {p99_pct}%"></div></div></td></tr>
<tr><td>Point Lookup P999</td><td class="good">{p999} µs</td><td><div class="bar"><div class="bar-fill" style="width: {p999_pct}%"></div></div></td></tr>
<tr><td>Batch Get P50</td><td>{batch_p50} µs</td><td><div class="bar"><div class="bar-fill" style="width: {batch_p50_pct}%"></div></div></td></tr>
<tr><td>Batch Get P99</td><td>{batch_p99} µs</td><td><div class="bar"><div class="bar-fill" style="width: {batch_p99_pct}%"></div></div></td></tr>
</table>

<h2>Throughput & Resources</h2>
<table>
<tr><th>Metric</th><th>Feather</th></tr>
<tr><td>Throughput</td><td class="good">{throughput:,} req/s</td></tr>
<tr><td>Memory Usage</td><td>{memory} MB</td></tr>
<tr><td>Cold Start</td><td class="good">{startup} ms</td></tr>
<tr><td>Binary Size</td><td>{binary} MB</td></tr>
</table>

<div class="note">
<strong>Methodology:</strong> Benchmarks run using <code>go test -bench</code> with {concurrency} concurrent goroutines
against {entities:,} entities with {features} features each. All tests run on the same hardware with
CGO_ENABLED=0 (pure Go, no C dependencies). Results are reproducible via
<code>./scripts/benchmark/run.sh</code>.
</div>

<footer>
Feather Feature Store — <a href="https://github.com/feather-store/feather" style="color: #58a6ff;">GitHub</a>
| Benchmarks are automated and updated weekly via CI.
</footer>
</body>
</html>"""

def main():
    results_path = os.path.join("docs", "benchmarks", "results.json")
    if not os.path.exists(results_path):
        print(f"Error: {results_path} not found. Run ./scripts/benchmark/run.sh first.")
        sys.exit(1)

    with open(results_path) as f:
        data = json.load(f)

    f = data["feather"]
    c = data["config"]
    max_latency = 10000  # scale bar to 10ms

    html = TEMPLATE.format(
        timestamp=data["timestamp"],
        entities=c["entities"], features=c["features"], concurrency=c["concurrency"],
        p50=f["point_lookup_p50_us"], p50_pct=min(100, f["point_lookup_p50_us"] * 100 / max_latency),
        p99=f["point_lookup_p99_us"], p99_pct=min(100, f["point_lookup_p99_us"] * 100 / max_latency),
        p999=f["point_lookup_p999_us"], p999_pct=min(100, f["point_lookup_p999_us"] * 100 / max_latency),
        batch_p50=f["batch_get_p50_us"], batch_p50_pct=min(100, f["batch_get_p50_us"] * 100 / max_latency),
        batch_p99=f["batch_get_p99_us"], batch_p99_pct=min(100, f["batch_get_p99_us"] * 100 / max_latency),
        throughput=f["throughput_rps"], memory=f["memory_mb"],
        startup=f["startup_ms"], binary=f["binary_size_mb"],
    )

    output_path = os.path.join("docs", "benchmarks", "index.html")
    with open(output_path, "w") as out:
        out.write(html)
    print(f"✅ Report generated: {output_path}")

if __name__ == "__main__":
    main()
