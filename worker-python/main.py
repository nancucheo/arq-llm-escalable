import asyncio
import json
import logging
import os
import random
import time

import redis.asyncio as aioredis

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

REDIS_HOST = os.getenv("REDIS_HOST", "localhost")
REDIS_PORT = int(os.getenv("REDIS_PORT", 6379))
QUEUE_KEY = "llm_queue"


async def simulate_llm(prompt: str) -> str:
    """Simulates LLM inference with a variable delay."""
    delay = random.uniform(10.0, 30.0)
    await asyncio.sleep(delay)
    words = prompt.split()
    response = (
        f"Respuesta generada para: '{prompt[:60]}{'...' if len(prompt) > 60 else ''}'. "
        f"[Simulado en {delay:.2f}s con {len(words)} palabras de entrada]"
    )
    return response


async def process_job(rdb: aioredis.Redis, raw: bytes) -> None:
    try:
        payload = json.loads(raw)
        job_id = payload["job_id"]
        client_id = payload["client_id"]
        prompt = payload["prompt"]
    except (json.JSONDecodeError, KeyError) as e:
        logger.error("malformed job payload: %s | error: %s", raw, e)
        return

    logger.info("processing job %s for client %s prompt: %s", job_id, client_id, prompt)
    start = time.perf_counter()

    try:
        result = await simulate_llm(prompt)
    except Exception as e:
        result = f"error: {e}"
        logger.exception("llm error for job %s", job_id)

    elapsed = time.perf_counter() - start
    response = json.dumps({"job_id": job_id, "result": result, "elapsed": round(elapsed, 3)})

    channel = f"result:{client_id}"
    await rdb.publish(channel, response)
    logger.info("published result for job %s to channel %s (%.3fs)", job_id, channel, elapsed)


async def worker_loop() -> None:
    rdb = await aioredis.from_url(
        f"redis://{REDIS_HOST}:{REDIS_PORT}",
        encoding="utf-8",
        decode_responses=False,
    )
    logger.info("worker connected to Redis at %s:%s", REDIS_HOST, REDIS_PORT)

    while True:
        try:
            # Blocking pop with 5s timeout so we can check for shutdown signals
            item = await rdb.blpop(QUEUE_KEY, timeout=5)
            if item is None:
                continue  # timeout, loop again
            _, raw = item
            asyncio.create_task(process_job(rdb, raw))
        except aioredis.RedisError as e:
            logger.error("redis error: %s — retrying in 2s", e)
            await asyncio.sleep(2)


if __name__ == "__main__":
    asyncio.run(worker_loop())
