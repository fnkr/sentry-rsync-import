# [sentry-rsync-import](https://github.com/fnkr/sentry-rsync-import)

Sometimes Sentry client and server cannot communicate directly with each other.
In this case it's possible to connect both parties using a custom transport.
This is a little tool that retrieves Sentry reports using rsync and then submits them to a Sentry server.
It is currently being used in production with ~600 events per minute.

## Define custom transport

```php
$sentryClient = new \Raven_Client($dsn);
$sentryClient->setTransport(function ($client, $data) {
    file_put_contents('/var/log/sentry/' . uniqid() . '.sentry_report', json_encode($data));
});
```
