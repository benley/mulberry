# Mulberry
A bare-bones TCP proxy for dynamic connection routing

## What does it do?
Mulberry listens on zero or more sockets, and any incoming connections are forwarded to the endpoint specified for that listen socket.

Mulberry responds to `SIGHUP` by re-reading its configuration and applying it, _without_ disrupting any existing connections whose parameters have not changed.

## What is it good for?
Mulberry is useful for building High-Availability (HA) systems\*.  (\*Some assembly required.)

The basic idea is:
- Run a daemon which monitors the health of your backends
- When the healthiest backend changes, have that daemon write an updated Mulberry config and send `SIGHUP` to the running `mulberry` process
