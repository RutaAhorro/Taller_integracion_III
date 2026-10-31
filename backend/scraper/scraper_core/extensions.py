import os

from scrapy import signals
from scrapy.exceptions import NotConfigured
from scraper_core.outbox import Outbox


class DurableCrawlExtension:
    """No considerar exitoso un proceso Scrapy que sólo devolvió exit code 0."""
    def __init__(self, crawler):
        self.crawler = crawler
        self.failed = False

    @classmethod
    def from_crawler(cls, crawler):
        if not os.getenv("SCRAPER_TRABAJO_ID") or not os.getenv("SCRAPER_STATE_DIR"):
            raise NotConfigured
        instance = cls(crawler)
        crawler.signals.connect(instance.on_error, signal=signals.item_error)
        crawler.signals.connect(instance.on_error, signal=signals.spider_error)
        crawler.signals.connect(instance.closed, signal=signals.spider_closed)
        return instance

    def on_error(self, **kwargs):
        self.failed = True

    def closed(self, spider, reason):
        spool = Outbox(os.environ["SCRAPER_TRABAJO_ID"], os.environ["SCRAPER_STATE_DIR"])
        try:
            success = reason == "finished" and not self.failed and not self.crawler.stats.get_value("retry/max_reached", 0)
            spool.set_progress("crawl_done", int(success))
        finally:
            spool.close()
