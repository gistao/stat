# 设计原则和背景
首先这个终究只是个统计打点库，它得特点是力求简单、高效、易用，且能覆盖大多数核心统计需求。此库在我们线上运行了几年，没出过问题，质量值得信赖。
# 功能列表
1.输出支持分组

分组是指将一系列指标输出到一行显示，格式支持json或者用逗号分割指标，可以方便的用ES、grafana或其他数据分析引擎做展示。

2.输出时间间隔可配

3.统计类型：qps；qpspeak(最大qps)；sum；val(无需加工的指标)

* qps：使用方只需++，库会精确的根据时间来计算qps

* qpspeak：使用方只需++，库会在生命周期内保留最大的qps来作为输出

* sum：使用方只需++，库会精确的进行sum运算

* val：使用方已计算好，或者无需再次加工的数，库必然会精确的输出...

4.指标的相关性趋势图(指标可配，且可多个指标同时显示在一张图上)
# 如何使用
example目录是示例代码
```
cd example
go run .
```

输出如下
```
2020-05-22 18:53:44.3112294 +0800 CST m=+1.055400101 auth: log=227573/s, psf_qps=72/s, pso=227573/s, psw=227572/s, LOGIN=229035, psf_sum=72, PSW=229034
2020-05-22 18:53:44.3671937 +0800 CST m=+1.111364401 gateway: close_qps=72/s, to=227572/s, wsof=682716/s, wsup=455144/s, 
close_qpk=72/s, close_sum=72, TO=229034, WSOF=687102, WSUP=458068, conn_val=229034

{"auth":{"qps":{"log":227573,"psf_qps":71,"pso":227573,"psw":227572},"sum":{"LOGIN":229035,"PSW":229034,"psf_sum":72}},"gateway":{"qps":{"close_qps":71,"to":227572,"wsof":682716,"wsup":455144},"qps_peak":{"close_qpk":71},"sum":{"TO":229034,"WSOF":687102,"WSUP":458068,"close_sum":72},"val":{"conn_val":229034}}}
2020-05-22 18:53:45.2916653 +0800 CST m=+2.035836001 auth: log=250385/s, psf_qps=72/s, pso=250385/s, psw=250386/s, LOGIN=479527, psf_sum=144, PSW=479527
2020-05-22 18:53:45.2926657 +0800 CST m=+2.036836401 gateway: close_qps=72/s, to=250386/s, wsof=751157/s, wsup=500771/s, 
close_qpk=72/s, close_sum=144, TO=479527, WSOF=1438581, WSUP=959054, conn_val=479526

{"auth":{"qps":{"log":250384,"psf_qps":71,"pso":250384,"psw":250385},"sum":{"LOGIN":479527,"PSW":479527,"psf_sum":144}},"gateway":{"qps":{"close_qps":71,"to":250385,"wsof":751156,"wsup":500771},"qps_peak":{"close_qpk":71},"sum":{"TO":479527,"WSOF":1438581,"WSUP":959054,"close_sum":144},"val":{"conn_val":479526}}}
2020-05-22 18:53:46.3010921 +0800 CST m=+3.045262801 auth: log=205616/s, psf_qps=71/s, pso=205616/s, psw=205615/s, LOGIN=686465, psf_sum=215, PSW=686464
2020-05-22 18:53:46.3020888 +0800 CST m=+3.046259501 gateway: close_qps=71/s, to=205615/s, wsof=616846/s, wsup=411231/s, 
close_qpk=72/s, close_sum=215, TO=686464, WSOF=2059392, WSUP=1372928, conn_val=686464

{"auth":{"qps":{"log":205616,"psf_qps":70,"pso":205616,"psw":205615},"sum":{"LOGIN":686465,"PSW":686464,"psf_sum":215}},"gateway":{"qps":{"close_qps":70,"to":205615,"wsof":616845,"wsup":411230},"qps_peak":{"close_qpk":71},"sum":{"TO":686464,"WSOF":2059392,"WSUP":1372928,"close_sum":215},"val":{"conn_val":686464}}}
```
