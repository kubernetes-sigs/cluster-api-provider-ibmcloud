# Regions-Zones Mapping

| Geography    | Location           | powervs regions |   powervs zones    | powervs service endpoint URL (public) | ibmcloud_vpc regions |           ibmcloud_vpc zones           | vpc service endpoint URL (public) | transit gateway locations |
|--------------|--------------------|:---------------:|:------------------:|:-------------------------------------:|:--------------------:|:--------------------------------------:|:---------------------------------:|:--------------------------|
| America      | Dallas, USA        |       dal       |       dal12        |     dal.power-iaas.cloud.ibm.com      |       us-south       | us-south-1<br>us-south-2<br>us-south-3 |    us-south.iaas.cloud.ibm.com    | us-south                  |
| America      | Dallas, USA        |    us-south     |      us-south      |   us-south.power-iaas.cloud.ibm.com   |       us-south       | us-south-1<br>us-south-2<br>us-south-3 |    us-south.iaas.cloud.ibm.com    | us-south                  |
| America      | Washington DC, USA |       wdc       |       wdc06        |     wdc.power-iaas.cloud.ibm.com      |       us-east        |  us-east-1<br>us-east-2<br>us-east-3   |    us-east.iaas.cloud.ibm.com     | us-east                   |
| America      | Washington DC, USA |     us-east     |      us-east       |   us-east.power-iaas.cloud.ibm.com    |       us-east        |  us-east-1<br>us-east-2<br>us-east-3   |    us-east.iaas.cloud.ibm.com     | us-east                   |
| America      | São Paulo, Brazil  |       sao       |       sao01        |     sao.power-iaas.cloud.ibm.com      |        br-sao        |    br-sao-1<br>br-sao-2<br>br-sao-3    |     br-sao.iaas.cloud.ibm.com     | br-sao                    |
| America      | Toronto, Canada    |       tor       |       tor01        |     tor.power-iaas.cloud.ibm.com      |        ca-tor        |    ca-tor-1<br>ca-tor-2<br>ca-tor-3    |     ca-tor.iaas.cloud.ibm.com     | ca-tor                    |
| America      | Montreal, Canada   |       mon       |       mon01        |     mon.power-iaas.cloud.ibm.com      |          -           |                   -                    |                 -                 | -                         |
| Europe       | Frankfurt, Germany |      eu-de      | eu-de-1<br>eu-de-2 |    eu-de.power-iaas.cloud.ibm.com     |        eu-de         |     eu-de-1<br>eu-de-2<br>eu-de-3      |     eu-de.iaas.cloud.ibm.com      | eu-de                     |
| Europe       | London, UK         |       lon       |   lon04<br>lon06   |     lon.power-iaas.cloud.ibm.com      |        eu-gb         |     eu-gb-1<br>eu-gb-2<br>eu-gb-3      |     eu-gb.iaas.cloud.ibm.com      | eu-gb                     |
| Europe       | Madrid             |       mad       |     mad02          |     mad.power-iaas.cloud.ibm.com      |        eu-es         |     eu-es-1<br>eu-es-2<br>eu-es-3      |     eu-es.iaas.cloud.ibm.com      | eu-es                     |
| Asia Pacific | Sydney, Australia  |       syd       |   syd04<br>syd05   |     syd.power-iaas.cloud.ibm.com      |        au-syd        |    au-syd-1<br>au-syd-2<br>au-syd-3    |     au-syd.iaas.cloud.ibm.com     | au-syd                    |
| Asia Pacific | Tokyo, Japan       |       tok       |       tok04        |     tok.power-iaas.cloud.ibm.com      |        jp-tok        |    jp-tok-1<br>jp-tok-2<br>jp-tok-3    |     jp-tok.iaas.cloud.ibm.com     | jp-tok                    |
| Asia Pacific | Osaka, Japan       |       osa       |       osa21        |     osa.power-iaas.cloud.ibm.com      |        jp-osa        |    jp-osa-1<br>jp-osa-2<br>jp-osa-3    |     jp-osa.iaas.cloud.ibm.com     | jp-osa                    | 


## References:
1. IBM Cloud doc for the PowerVS [locations][powervs-locations]
2. IBM Cloud doc for the VPC [Regions][vpc-locations]
3. IBM Cloud doc for Transit Gateway [locations][transit-gateway-locations]


[powervs-locations]: https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server
[vpc-locations]: https://cloud.ibm.com/docs/vpc?topic=vpc-creating-a-vpc-in-a-different-region&interface=cli
[transit-gateway-locations]: https://cloud.ibm.com/docs/transit-gateway?topic=transit-gateway-tg-locations