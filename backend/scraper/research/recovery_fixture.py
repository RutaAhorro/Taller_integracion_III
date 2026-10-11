"""Crawl de prueba sin red externa, usando pipeline y señales reales de Scrapy."""
from scrapy import Spider
from scrapy.crawler import CrawlerProcess
from scrapy.settings import Settings


class RecoveryFixture(Spider):
    name = 'recovery_fixture'
    start_urls = ['data:text/plain,fixture']

    def parse(self, response):
        for sku in ['SUP270-SCRAPY-1','SUP270-SCRAPY-2','SUP270-SCRAPY-1']:
            yield {'sku':sku,'producto':'Fixture local SUP270','precio_normal':1100,'supermercado':'Jumbo'}


if __name__ == '__main__':
    settings = Settings()
    settings.setmodule('scraper_core.settings')
    settings.set('ROBOTSTXT_OBEY',False)
    settings.set('DOWNLOAD_DELAY',0)
    settings.set('AUTOTHROTTLE_ENABLED',False)
    settings.set('LOG_ENABLED',False)
    settings.set('SCRAPER_API_BATCH_SIZE',2)
    process = CrawlerProcess(settings)
    process.crawl(RecoveryFixture)
    process.start()
