small-proxy
=======
[![License](https://img.shields.io/badge/license-apache2-blue.svg)](LICENSE)

**small-proxy是使用反向代理+TCP隧道实现的域名式访问内网穿透,类似花生壳和ngrok代理软件,可用于微信平台和需要域名授权的开发中使用。**

> ⚠ small-proxy目前只支持TCP和http代理,暂不支持其他协议。

## 特点

- 域名式访问内网穿透
- 跨平台(支持6大操作系统)
- 易学习(代码量小)

## 使用教程

由于篇幅和图片偏多,请移步到本篇文章:[Golang实现的域名式访问内网穿透](http://www.5lazy.cn/post-145.html)。

## 注意事项

由于github上的目录与go的目录结构有一定冲突,不能使用go get命令进行安装,建议使用git下载或者手动下载源代码,然后执行go build进行编译安装。

## License

Apache License Version 2.0 see http://www.apache.org/licenses/LICENSE-2.0.html
