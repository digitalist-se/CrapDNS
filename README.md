

<!-- toc -->

- [CrapDNS](#crapdns)
  * [Why on earth would you want that?](#why-on-earth-would-you-want-that)
  * [How it works](#how-it-works)
  * [Example](#example)
  * [Known Issues](#known-issues)

<!-- tocstop -->

# CrapDNS

A "DNS server" in Golang for MacOS only that gets it's name from the implementation the  
**C**ommon **R**equest and **A**nswer **P**rotcol.

Only joking, it gets it's name from being a bit crap. :)  

This program exists for one reason only: to return DNS A records in it's "Zones" that always point to 127.0.0.1 

## Why on earth would you want that?
Many developers on MacOS use tools like [Lando](https://docs.devwithlando.io/) to manage development with Docker. This is a fantastic tool, 
but if you want to use it offline, or with custom domains, then you have to either mess around with your hosts file or mess 
around with dnsmasq. 
All very nice, but wouldn't it be nicer just to type:  
```
sudo crapdns --domains=test,dev,local
```
and all your *.test, *.dev and *.local environments would automatically resolve?

## How it works
CrapDNS makes use of the MacOS default resolver facility that allows entries in the directory `/etc/resolver/` to specify 
name servers. When you run CrapDNS it makes an entry for each domain given of the form `/etc/resolver/<domain>` with a 
file which contains 
```
###CRAPDNS###
nameserver 127.0.0.1
```

Thereafter MacOS will automatically lookup the domain ( and subdomains ) on this address.  
CrapDNS also runs a pseudo DNS server on port 53 on the address 127.0.0.1 which always returns an A record pointing to 
127.0.0.1 if the domain is in it's "zones" or NXDOMAIN otherwise.  
All resolvers are removed at program exit.

## Example
You project environment lives on the domain `foo.test`  

Start CrapDNS
```
sudo crapdns --domains=test
```
Start Lando  
```
lando poweroff && LANDO_PROXY_DOMAIN=test lando start
```
$$$PROFIT$$$

CrapDNS may also be run without parameters or with the parameter `--configfile=<configfile>` (default crapdns.conf)
which will read a list of domain names, one-per-line from a config file.  

## Known Issues
* CrapDNS needs to run as superuser to bind to port 53 and manipulate the resolver. It does __not__ drop privileges after 
binding to the port, basically because I cannot figure out how to do so reliably in Golang.
Theoretically this could be a security issue, but since we only bind to the loopback address, it is unlikely to be an attack-vector.
* No guarantee is given that this programme will not eat your firstborn, or take a dump in your morning coffee. Use at your own risk.


Comments, pull-requests, enhancements are very welcome.