# ip2region 离线数据库

本目录随 Lite SSO 发布，用于扫码确认和登录设备列表的 IP 地点展示。使用官方免费 IPv4、IPv6 数据，不调用外部定位 API。

- 项目：https://github.com/lionsoul2014/ip2region
- 固定版本：v3.18.0
- 对应提交：c1a1fc7d5941760db3f8431dc05c48cf7f0e30a1
- Go 绑定：v0.0.0-20260901011515-c1a1fc7d5941
- 原始文件：该提交下的 `data/ip2region_v4.xdb` 和 `data/ip2region_v6.xdb`
- 许可证：随附上游 `LICENSE.md`（Apache-2.0 OR MIT）；数据精度和更新说明参见上游 README。

## 使用

在仓库根目录启动服务，默认读取本目录。可通过 `geoip.data_dir` 或 `GEOIP_DATA_DIR` 设置目录；必须同时提供两个 xdb 文件。启动时校验格式，缺失或损坏直接报错。查询使用共享的 vector index 缓存，每个协议有 4 个读取器。

查询结果只展示国家、省份和城市，忽略运营商与国家代码，过滤 `0` 和空值，并合并重复名称。无效 IP、内网 IP、查询失败或无结果返回空字符串，客户端显示“未知”。这是公网出口的大致地点，不代表用户的精确位置。

## 手动更新

1. 从上游一个固定发布版本取得两个 xdb 文件，保留上游许可证，并将 Go 绑定更新到同一提交。
2. 替换本目录文件，更新上面的版本与提交以及下方校验和。
3. 运行 `go test ./util/ipgeo ./service/auth`，确认双协议查询和地点快照测试通过。
4. 重新构建并重启服务；Dockerfile 和 Dockerfile.vercel 均复制本目录，已有 session 的地点快照不变。

不在启动时或请求中下载数据，不进行历史地点回填。

## SHA-256

```text
8e31bbdccb5bf21028af10592d4312ec975da0bffa108c0c5d862a12190f9ad3  ip2region_v4.xdb
939f6b46bd2b8bec3cf7c5ceb8ba782266ae9b1f35b5ba7916700dec0b7506ed  ip2region_v6.xdb
```
