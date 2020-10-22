# AXL Tool

Issue AXL requests to Cisco Unified Communications Manager (CUCM).

Usage example:

```xml
addRP.xml:
<addRoutePartition>
  <routePartition>
    <name>AXL_PT</name>
  </routePartition>
</addRoutePartition>
```

`axl -cucm 10.10.20.1 -u axladmin -p cisco123 -xml addRP.xml`

A few things to point out:
* if you want to ignore the TLS certificate, use the `-k` option
* default AXL schema is 12.5, it can be changed with `-s 10.0` for example
* the AXL method is derived from the top-level XML element (`addRoutePartition`) in this example

By specifying a CSV file, you can run AXL requests in bulk. Usage example:

```xml
addRPbulk.xml:
<addRoutePartition>
  <routePartition>
    <name>{{var 0 log}}</name>
    <description>{{var 1}}</description>
  </routePartition>
</addRoutePartition>
```

```csv
rp.csv:
PT_One,First Partition
PT_Two,Second Partition
PT_Three,Third Partition
```

`axl -cucm 10.10.20.1 -u axladmin -p cisco123 -xml addRPbulk.xml -csv rp.csv`

In bulk mode, one XML request is sent for each CSV line. The individual values from your CSV are inserted using the `{{var n}}` syntax, where n refers to the CSV column, starting at 0 for the first column. If you add the `log` parameter (e.g. `{{var 0 log}}`) the corresponding value will be added in the output log.
