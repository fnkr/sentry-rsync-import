# [sentry-rsync-import](https://github.com/fnkr/sentry-rsync-import)

Sometimes Sentry client and server cannot communicate directly with each other.
It is still possible to forward reports to Sentry using custom transports.
This is a tool to download serialized reports from a server using rsync and submit them to Sentry.
Is is being used in production with ~600 events per minute.

## Configure custom transport

### Python

```python
import logging
import uuid
import zlib
from raven import Client as Sentry
from raven.transport.base import Transport as SentryTransport


class SentrySaveFileTransport(SentryTransport):
    def send(self, url, data, headers):
        with open('/var/log/sentry/{}.sentry_report'.format(str(uuid.uuid4())), 'w') as report:
            report.write(zlib.decompress(data).decode('utf8'))


logging.basicConfig(format="%(asctime)s %(levelname)s %(message)s")

sentry = Sentry(dsn, transport=SentrySaveFileTransport)
```

### PHP

```php
$sentryClient = new \Raven_Client($dsn);
$sentryClient->setTransport(function ($client, $data) {
    file_put_contents('/var/log/sentry/' . uniqid() . '.sentry_report', json_encode($data));
});
```
