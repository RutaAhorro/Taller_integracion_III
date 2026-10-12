import os

# Scrapy settings for scraper_core project
#
# For simplicity, this file contains only settings considered important or
# commonly used. You can find more settings consulting the documentation:
#
#     https://docs.scrapy.org/en/latest/topics/settings.html
#     https://docs.scrapy.org/en/latest/topics/downloader-middleware.html
#     https://docs.scrapy.org/en/latest/topics/spider-middleware.html

BOT_NAME = "scraper_core"

SPIDER_MODULES = ["scraper_core.spiders"]
NEWSPIDER_MODULE = "scraper_core.spiders"

ADDONS = {}
EXTENSIONS = {"scraper_core.extensions.DurableCrawlExtension": 500}


# Crawl responsibly by identifying yourself (and your website) on the user-agent
#USER_AGENT = "scraper_core (+http://www.yourdomain.com)"

# Obey robots.txt rules
ROBOTSTXT_OBEY = True

# Concurrency and throttling settings
# El scraper debe comportarse de forma ética y conservadora para evitar
# bloquear el sitio objetivo y sobrecargar el contenedor del proyecto.
CONCURRENT_REQUESTS = 1
CONCURRENT_REQUESTS_PER_DOMAIN = 1
DOWNLOAD_DELAY = 2
RANDOMIZE_DOWNLOAD_DELAY = False
DOWNLOAD_TIMEOUT = 30
DOWNLOAD_MAXSIZE = 5 * 1024 * 1024
AUTOTHROTTLE_ENABLED = True
AUTOTHROTTLE_START_DELAY = 2
AUTOTHROTTLE_MAX_DELAY = 60
AUTOTHROTTLE_TARGET_CONCURRENCY = 0.5
RETRY_ENABLED = True
RETRY_TIMES = 3
RETRY_BACKOFF_FACTOR = 2
RETRY_BACKOFF_MAX = 120
RETRY_HTTP_CODES = [429, 500, 503, 504]

# Actívalo con SCRAPY_JOBDIR sólo cuando quieras reanudar una ejecución.
JOBDIR = os.getenv("SCRAPY_JOBDIR")

# Disable cookies (enabled by default)
#COOKIES_ENABLED = False

# Disable Telnet Console (enabled by default)
#TELNETCONSOLE_ENABLED = False

# Override the default request headers:
#DEFAULT_REQUEST_HEADERS = {
#    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
#    "Accept-Language": "en",
#}

# Enable or disable spider middlewares
# See https://docs.scrapy.org/en/latest/topics/spider-middleware.html
#SPIDER_MIDDLEWARES = {
#    "scraper_core.middlewares.ScraperCoreSpiderMiddleware": 543,
#}

# Enable or disable downloader middlewares
# See https://docs.scrapy.org/en/latest/topics/downloader-middleware.html
#DOWNLOADER_MIDDLEWARES = {
#    "scraper_core.middlewares.ScraperCoreDownloaderMiddleware": 543,
#}

# Enable or disable extensions
# See https://docs.scrapy.org/en/latest/topics/extensions.html
#EXTENSIONS = {
#    "scrapy.extensions.telnet.TelnetConsole": None,
#}

# Configure item pipelines
# See https://docs.scrapy.org/en/latest/topics/item-pipeline.html
SCRAPER_API_URL = os.getenv(
	"SCRAPER_API_URL",
	"http://localhost:8082/api/v1/scraper/productos",
)
SCRAPER_API_TIMEOUT = float(os.getenv("SCRAPER_API_TIMEOUT", "30"))
SCRAPER_API_BATCH_SIZE = int(os.getenv("SCRAPER_API_BATCH_SIZE", "100"))
ITEM_PIPELINES = {
	"scraper_core.pipelines.ScraperCorePipeline": 300,
}

# Enable and configure the AutoThrottle extension (disabled by default)
# See https://docs.scrapy.org/en/latest/topics/autothrottle.html
# Enable showing throttling stats for every response received:
#AUTOTHROTTLE_DEBUG = False

# Enable and configure HTTP caching (disabled by default)
# See https://docs.scrapy.org/en/latest/topics/downloader-middleware.html#httpcache-middleware-settings
#HTTPCACHE_ENABLED = True
#HTTPCACHE_EXPIRATION_SECS = 0
#HTTPCACHE_DIR = "httpcache"
#HTTPCACHE_IGNORE_HTTP_CODES = []
#HTTPCACHE_STORAGE = "scrapy.extensions.httpcache.FilesystemCacheStorage"

# Set settings whose default value is deprecated to a future-proof value
FEED_EXPORT_ENCODING = "utf-8"
