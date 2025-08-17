# Guide

## **install go enviroment**

use gvm as go version managerment see from [see](https://github.com/moovweb/gvm)

sudo apt-get install bison

1. Install [Bison](https://www.gnu.org/software/bison/):

   `sudo apt-get install bison`
2. Install gvm:

   `bash < <(curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer)`
3. Install go:

   `gvm install go1.24.4`
   `gvm use go1.24.4 --default`

## **build and run**

   add to your vscode setting.json

```
    "go.gopath": "/home/ryan/.gvm/pkgsets/go1.24.4/global",
    "go.goroot": "/home/ryan/.gvm/gos/go1.24.4",
```

   `make build`
