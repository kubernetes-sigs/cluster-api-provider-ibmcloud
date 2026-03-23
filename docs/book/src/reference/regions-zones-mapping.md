# Regions-Zones Mapping

| Geography     | Location           | PowerVS Regions |         PowerVS Zones          | PowerVS Service Endpoint URL (Public) | IBMCLOUD_VPC Regions |           IBMCLOUD_VPC Zones           | VPC Service Endpoint URL (Public) | Transit Gateway Locations |
| ------------- | ------------------ | :-------------: | :----------------------------: | :-----------------------------------: | :------------------: | :------------------------------------: | :-------------------------------: | :-----------------------: |
| North America | Dallas, USA        |    us-south     | us-south<br>dal10<br>dal12<br>dal14 |   us-south.power-iaas.cloud.ibm.com   |       us-south       | us-south-1<br>us-south-2<br>us-south-3 |    us-south.iaas.cloud.ibm.com    |         us-south          |
| North America | Washington DC, USA |     us-east     |    us-east<br>wdc06<br>wdc07   |   us-east.power-iaas.cloud.ibm.com    |       us-east        |  us-east-1<br>us-east-2<br>us-east-3   |    us-east.iaas.cloud.ibm.com     |          us-east          |
| North America | Toronto, Canada    |       tor       |             tor01              |     tor.power-iaas.cloud.ibm.com      |        ca-tor        |    ca-tor-1<br>ca-tor-2<br>ca-tor-3    |     ca-tor.iaas.cloud.ibm.com     |          ca-tor           |
| North America | Montreal, Canada   |       mon       |             mon01              |     mon.power-iaas.cloud.ibm.com      |        ca-mon        |    ca-mon-1<br>ca-mon-2<br>ca-mon-3    |     ca-mon.iaas.cloud.ibm.com     |             -             |
| South America | São Paulo, Brazil  |       sao       |    sao01<br>sao04<br>sao05     |     sao.power-iaas.cloud.ibm.com      |        br-sao        |    br-sao-1<br>br-sao-2<br>br-sao-3    |     br-sao.iaas.cloud.ibm.com     |          br-sao           |
| Europe        | Frankfurt, Germany |      eu-de      |       eu-de-1<br>eu-de-2       |    eu-de.power-iaas.cloud.ibm.com     |        eu-de         |     eu-de-1<br>eu-de-2<br>eu-de-3      |     eu-de.iaas.cloud.ibm.com      |           eu-de           |
| Europe        | London, UK         |       lon       |         lon04<br>lon06         |     lon.power-iaas.cloud.ibm.com      |        eu-gb         |     eu-gb-1<br>eu-gb-2<br>eu-gb-3      |     eu-gb.iaas.cloud.ibm.com      |           eu-gb           |
| Europe        | Madrid             |       mad       |         mad02<br>mad04         |     mad.power-iaas.cloud.ibm.com      |        eu-es         |     eu-es-1<br>eu-es-2<br>eu-es-3      |     eu-es.iaas.cloud.ibm.com      |           eu-es           |
| Asia Pacific  | Sydney, Australia  |       syd       |         syd04<br>syd05         |     syd.power-iaas.cloud.ibm.com      |        au-syd        |    au-syd-1<br>au-syd-2<br>au-syd-3    |     au-syd.iaas.cloud.ibm.com     |          au-syd           |
| Asia Pacific  | Tokyo, Japan       |       tok       |             tok04              |     tok.power-iaas.cloud.ibm.com      |        jp-tok        |    jp-tok-1<br>jp-tok-2<br>jp-tok-3    |     jp-tok.iaas.cloud.ibm.com     |          jp-tok           |
| Asia Pacific  | Osaka, Japan       |       osa       |             osa21              |     osa.power-iaas.cloud.ibm.com      |        jp-osa        |    jp-osa-1<br>jp-osa-2<br>jp-osa-3    |     jp-osa.iaas.cloud.ibm.com     |          jp-osa           |
| Asia Pacific  | Chennai, India     |       che       |    che01<br>che02<br>che03     |     che.power-iaas.cloud.ibm.com      |        in-che        |    in-che-1<br>in-che-2<br>in-che-3    |     in-che.iaas.cloud.ibm.com     |          in-che           |
| Asia Pacific  | Mumbai, India      |        -        |               -                |                   -                   |        in-mum        |    in-mum-1<br>in-mum-2<br>in-mum-3    |     in-mum.iaas.cloud.ibm.com     |          in-mum           |

## References:
1. IBM Cloud Documentation for:
    * [PowerVS Locations][powervs-locations]
    * [VPC Regions][vpc-locations]
    * [Transit Gateway][transit-gateway-locations]
4. Deploy on IBM Cloud:
    * [PowerVS][deploy-powervs]
    * [VPC][deploy-vpc]
    * [Transit Gateway][deploy-transit-gateway]


[powervs-locations]: https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server
[vpc-locations]: https://cloud.ibm.com/docs/vpc?topic=vpc-creating-a-vpc-in-a-different-region&interface=cli
[transit-gateway-locations]: https://cloud.ibm.com/docs/transit-gateway?topic=transit-gateway-tg-locations
[deploy-powervs]: https://cloud.ibm.com/power/workspaces
[deploy-vpc]: https://cloud.ibm.com/infrastructure/compute/vs
[deploy-transit-gateway]: https://cloud.ibm.com/interconnectivity/transit