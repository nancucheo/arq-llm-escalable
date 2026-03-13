#!/usr/bin/env python3
"""
Load test script for the LLM Gateway WebSocket endpoint.

Usage:
    pip install websockets
    python scripts/load_test.py --total 1000 --concurrency 1000
    python scripts/load_test.py --total 10 --concurrency 10  # smoke test
"""

import argparse
import asyncio
import json
import statistics
import time

import websockets


def parse_args():
    parser = argparse.ArgumentParser(description="WebSocket load tester for LLM Gateway")
    parser.add_argument("--url", default="ws://localhost:8080/ws", help="WebSocket URL")
    parser.add_argument("--total", type=int, default=1000, help="Total number of messages to send")
    parser.add_argument("--concurrency", type=int, default=1000, help="Max simultaneous WebSocket connections")
    parser.add_argument("--prompt", default=None, help="Custom prompt text (default: 'Load test #N')")
    return parser.parse_args()


async def run_single(index, url, prompt_text, semaphore, results, progress):
    async with semaphore:
        start = time.monotonic()
        prompt = prompt_text if prompt_text else f"Load test #{index}"
        outcome = "error"
        latency = None

        try:
            async with websockets.connect(url, open_timeout=10) as ws:
                await ws.send(json.dumps({"prompt": prompt}))
                raw = await ws.recv()
                latency = time.monotonic() - start

                data = json.loads(raw)
                if "error" in data:
                    outcome = "timeout"
                else:
                    outcome = "success"

        except Exception:
            latency = time.monotonic() - start

        results.append((outcome, latency))

        done = len(results)
        progress["count"] = done
        if done % 50 == 0 or done == progress["total"]:
            errors = sum(1 for r in results if r[0] == "error")
            print(f"[Progress] {done}/{progress['total']} done | {errors} errors")


async def main():
    args = parse_args()
    semaphore = asyncio.Semaphore(args.concurrency)
    results = []
    progress = {"count": 0, "total": args.total}

    print(f"Starting load test: {args.total} messages, concurrency={args.concurrency}, url={args.url}")
    wall_start = time.monotonic()

    tasks = [
        run_single(i + 1, args.url, args.prompt, semaphore, results, progress)
        for i in range(args.total)
    ]
    await asyncio.gather(*tasks)

    wall_time = time.monotonic() - wall_start

    successes = [(o, l) for o, l in results if o == "success"]
    timeouts  = [(o, l) for o, l in results if o == "timeout"]
    errors    = [(o, l) for o, l in results if o == "error"]

    n_total   = len(results)
    n_success = len(successes)
    n_timeout = len(timeouts)
    n_error   = len(errors)

    pct = lambda n: f"{n / n_total * 100:.1f}%" if n_total else "0%"

    print()
    print("=== Load Test Results ===")
    print(
        f"Total: {n_total:>6}  |  "
        f"Success: {n_success} ({pct(n_success)})  |  "
        f"Timeout: {n_timeout} ({pct(n_timeout)})  |  "
        f"Error: {n_error} ({pct(n_error)})"
    )

    if successes:
        latencies = sorted(l for _, l in successes)
        n = len(latencies)
        p = lambda pct_val: latencies[min(int(n * pct_val / 100), n - 1)]

        print(
            f"Latency (successful): "
            f"min={min(latencies):.1f}s  "
            f"avg={statistics.mean(latencies):.1f}s  "
            f"p50={p(50):.1f}s  "
            f"p95={p(95):.1f}s  "
            f"p99={p(99):.1f}s  "
            f"max={max(latencies):.1f}s"
        )
    else:
        print("Latency (successful): N/A — no successful responses")

    throughput = n_total / wall_time if wall_time > 0 else 0
    print(f"Wall time: {wall_time:.1f}s  |  Throughput: {throughput:.1f} completions/sec")


if __name__ == "__main__":
    asyncio.run(main())
