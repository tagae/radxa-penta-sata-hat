Radxa Penta SATA HAT Top Board Controller
=========================================

Build service:

```shell
make build
```

Build package:

```shell
make deb
```

Build for Raspberry Pi 32-bit:

```shell
make deb ARCH=armhf
```

Build with a custom version:

```shell
make deb VERSION=0.4
```

Run the controller locally:

```shell
env $(cat env/rpi5.env | grep -v '^#') go run . serve
```


Vendor Documentation
--------------------

[Penta SATA HAT wiki](<https://wiki.radxa.com/Penta_SATA_HAT>).


### 40-Pin GPIO Header Pinout

| Description          | Function | Pin# | Pin# | Function   | Description |
|----------------------|----------|-----:|-----:|------------|-------------|
|                      |          |    1 |    2 | VCC5V0_SYS |             |
| OLED I2C             | I2C_SDA  |    3 |    4 | VCC5V0_SYS |             |
| OLED I2C             | I2C_SCL  |    5 |    6 |            |             |
|                      |          |    7 |    8 |            |             |
|                      |          |    9 |   10 |            |             |
| top board key        | GPIO4_C2 |   11 |   12 |            |             |
|                      | GPIO4_C6 |   13 |   14 |            |             |
|                      |          |   15 |   16 | GPIO4_D2   | reset OLED  |
|                      |          |   17 |   18 |            |             |
|                      |          |   19 |   20 |            |             |
|                      |          |   21 |   22 |            |             |
|                      |          |   23 |   24 |            |             |
|                      |          |   25 |   26 | ADC_IN0    | temperature |
|                      | SDA      |   27 |   28 | SCL        |             |
|                      |          |   29 |   30 |            |             |
|                      |          |   31 |   32 |            |             |
| control tb-fan speed | PWM_33   |   33 |   34 |            |             |
|                      |          |   35 |   36 |            |             |
|                      |          |   37 |   38 |            |             |
|                      |          |   39 |   40 |            |             |


### 2x5 PHD 2.0mm Connector Pinout

| Function | Pin# | Pin# | Function              |
|----------|-----:|-----:|-----------------------|
| I2C_SDA  |    1 |    2 | VCC3V3_SYS            |
| I2C_SCL  |    3 |    4 | VCC5V0_SYS            |
| GPIO4_D2 |    5 |    6 | GPIO4_C2              |
| GND      |    7 |    8 | PWM_33 OR GPIO4_C6    |
| GND      |    9 |   10 | NC                    |

Pin 8 support 2 different signals. Please check your board against the component placement map to find out which signal is routed.

If you have R563 populated on your board, you have GPIO4_C6 on pin 8.

If you have R573 populated on your board, you have PWM_33 on pin 8.

Both resistors are located on the left bottom corner of the map.

Do not populate both resistors!
