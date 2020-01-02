# osbuild-composer

An HTTP service for building bootable OS images. It provides the same API as [lorax-composer](https://github.com/weldr/lorax) but in the background it uses [osbuild](https://github.com/osbuild/osbuild) to create the images.

You can control it in [Cockpit](https://github.com/weldr/cockpit-composer) or using the [composer-cli](https://weldr.io/lorax/composer-cli.html). To get started on Fedora, run:

```
# dnf install cockpit-composer golang-github-osbuild-composer composer-cli
# systemctl enable --now cockpit.socket
# systemctl enable --now osbuild-composer.socket
```

Now you can access the service using `composer-cli`, for example:

```
composer-cli status show
```

or using a browser: `http://localhost:9090`

## API documentation

Please refer to the [lorax-composer](https://github.com/weldr/lorax)'s documenation as osbuild-composer is a drop-in replacement.

