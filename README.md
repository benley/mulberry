# Mulberry
A bare-bones TCP proxy for dynamic connection routing

## What does it do?
Mulberry listens on zero or more sockets, and any incoming connections are forwarded to the endpoint specified for that listen socket.

Mulberry responds to `SIGHUP` by re-reading its configuration and applying it, _without_ disrupting any existing connections whose parameters have not changed.

## What is it good for?
Mulberry is useful for building High-Availability (HA) systems\*.  (\*Some assembly required.)

The basic idea is:
- Run `mulberry`
- Run a daemon which monitors the health of your backends
- When the healthiest backend changes, have that daemon write an updated Mulberry config to disk and send `SIGHUP` to the running instance of `mulberry`

For advanced users:
- Generate a GPG keypair with `gpg --homedir /path/to/keyring --gen-key`
- Run `mulberry -config /some/writable/path/config.yaml -keyring /path/to/keyring/pubring.gpg`
- Monitor your backends
- When the healthiest backend changes, run `mulberrypush -config /path/to/new/config.yaml -url http://hostname:8643/upload -keyring secring.gpg -keyid fingerprint`; repeat for each host with a running `mulberry` instance
