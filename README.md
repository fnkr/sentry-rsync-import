# sentry-rsync-import

Sometimes Sentry client and server cannot communicate directly with each other.
In this case it's possible to connect both parties using a custom transport.
This is a little tool that fetches Sentry reports via rsync and forwards them to a specific Sentry server.

I wrote this in a train with little or no internet connection, its almost completely untested, possibly kills your cat and stuff (see LICENSE).
It's also not that complicated, basically it's just rsync and curl.

## Define custom transport

```php
$sentryClient = new \Raven_Client($dsn);
$sentryClient->setTransport(
    'transport' => function($client, $data) {
        file_put_contents('/var/log/sentry/' . uniqid() . '.sentry_report', json_encode($data));
    }
);
```
