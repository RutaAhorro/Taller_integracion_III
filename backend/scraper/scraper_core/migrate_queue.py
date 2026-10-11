"""Migrar la lista anterior sin retirarla antes de confirmar la outbox SQL.

Ejecutar sólo con productores y worker anteriores detenidos.
"""
import argparse
import json
import os
from urllib.error import HTTPError
from urllib.request import Request
from uuid import UUID

from redis import Redis
from scraper_core.api_client import ScraperAPIClient
from scraper_core.worker import ALLOWED_SPIDERS


def migrate_one(redis, api, *, apply=False):
    raw = redis.lindex('scraper:jobs', -1)
    if raw is None:
        return False
    job = json.loads(raw)
    work_id = str(UUID(job['trabajo_id']))
    spider = job.get('spider','jumbo_rsc')
    if spider not in ALLOWED_SPIDERS:
        raise ValueError('Spider inválido; se conserva la entrada anterior')
    data = {'spider':spider}
    for old,new in [('product_url','producto_url'),('product_id','producto_id')]:
        if old in job: data[new]=job[old]
    if apply:
        request = Request(api.endpoint.removesuffix('/productos') + '/trabajos/' + work_id + '/ejecutar',
                          data=json.dumps(data).encode(),method='POST',headers={'Content-Type':'application/json'})
        try:
            with api.opener(request,timeout=api.timeout) as response:
                result=json.loads(response.read().decode('utf-8'))
                if response.status != 202 or result.get('trabajo_id')!=work_id or response.headers.get('X-Scraper-Queue')!='durable-stream':
                    raise RuntimeError('No se confirmó la outbox SQL; se conserva la entrada anterior')
        except HTTPError as error:
            if error.code != 409 or api.get_work(work_id)['estado'] not in ('completado','fallido'):
                raise
        redis.lrem('scraper:jobs',1,raw)
    return work_id


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    mode=parser.add_mutually_exclusive_group(required=True)
    mode.add_argument('--dry-run',action='store_true')
    mode.add_argument('--apply',action='store_true')
    args=parser.parse_args()
    redis=Redis.from_url(os.environ['REDIS_URL'],decode_responses=True,socket_timeout=5)
    api=ScraperAPIClient(os.environ['SCRAPER_API_URL'])
    if args.dry_run:
        print(json.dumps({'legacy_pending':redis.llen('scraper:jobs'),'oldest_work_id':migrate_one(redis,api)}))
    else:
        count=0
        while migrate_one(redis,api,apply=True): count+=1
        print(json.dumps({'migrated':count,'legacy_pending':redis.llen('scraper:jobs')}))


if __name__=='__main__':
    main()
