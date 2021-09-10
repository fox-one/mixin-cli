# mixin-cli
Interactive command-line applications to manage mixin dapps

## Install

### From Source Code

```bash
$ git clone git@github.com:fox-one/mixin-cli.git & cd mixin-cli
$ go install
```

## KeyStore

### Format

```json5
{
  "client_id": "",
  "session_id": "",
  "private_key": "",
  "pin_token": "",
  "pin": "", // optional
}
```

### Select KeyStore File

1. ```mixin-cli a``` load ~/.mixin-cli/a.json or ~/mixin-cli/*/a.json
2. ```mixin-cli ./a.json``` load from given path

## Mode

```mixin-cli a``` will load ~/.mixin-cli/a.json and enter interactive mode, use quit/q to quit.

```mixin-cli a me``` will load ~/.mixin-cli/a.json and execute command ```me``` directly.

## Commands

### assets

Output assets, only show assets with balance by default.
Use -a to show all assets.

```bash
$ assets

Asset ID                             Symbol Name             Price(USD) Change  Balance Value(USD)
c6d0c728-2624-429b-8e0d-d9d19b6592fa BTC    Bitcoin          9929.61    -3.14%  0       0.00
fd11b6e3-0b87-41f1-a41f-f0e9b49e5bf0 BCH    Bitcoin Cash     445.04     -8.17%  0       0.00
Total Values: 0 USD
```

```bash
$ assets btc
$ assets c6d0c728-2624-429b-8e0d-d9d19b6592fa

Asset ID    c6d0c728-2624-429b-8e0d-d9d19b6592fa
Symbol      BTC
Name        Bitcoin
Balance     0
Price(USD)  9924.06
Change      -3.14%
Value(USD)  0.00
Destination 1BDD5mPA3nWnDBtfBshuWuovpRR4uTbh9p
Tag
Logo        https://mixin-images.zeromesh.net/HvYGJsV5TGeZ-X9Ek3FEQohQZ3fE9LBEBGcOcn4c4BNHovP4fW4YB97Dg5LcXoQ1hUjMEgjbl1DPlKg1TW7kK6XP=s128
```

### me

Output dapp's profile

### deposit

Output deposit qrcode.

### pay

Transfer to mixin user

```bash
$ pay 8017d200-7870-4b82-b53f-74bae1d2dad7 c6d0c728-2624-429b-8e0d-d9d19b6592fa 0.01 "pay by mixin cli"

Pay 0.01 BTC to yiplee
Continue: y█
Error: Insufficient balance. [202/20117]
```

### pin

Update pin

```bash
$ pin new_pin
```

### sign

Sign url with private key

```bash
$ sign /assets --exp 1h

sign /assets with exp duration 1h0m0s

eyJhbGciOiJSUzUxMiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1ODE4MjM3OTQsImlhdCI6MTU4MTgyMDE5NCwianRpIjoiNzQyYzQ2OWYtNGM1My00NmM2LThiMGEtYjhjZjQ5MWUxYTFlIiwic2NwIjoiRlVMTCIsInNpZCI6ImRiMmYzMmJiLWYyYTUtNDJiMS1iOTQ2LTYzYTRlMTI5YjAyYyIsInNpZyI6Ijg1NzNlYzVhNDdjNjkxZGIzMDczZjkyMjUwNjg3OTk4OWJhYTIwYjgyZmNkMWUxMjZjMDdkYjZiNGI5ZTA3OWUiLCJ1aWQiOiI1YzRmMzBhNi0xZjQ5LTQzYzMtYjM3Yi1jMDFhYWU1MTkxYWYifQ.i2H1AaCSXw5F7rA0iyqHqQxQP34uoecnWEbH-cwfFegtBnjYq1jxAgYNnMautH9_zJbnJ9yHIeDZ80UK7KVDpLz61k0k27tHsPJt8yPFaC5aoW_r3PiqlUIYW59c_tm42IrD_SzNMRGJ_JCQXHr9fU42VyRLaN0A--8TRFWzG6A
```

### search user

Search user by identity number or mixin id

```bash
$ search user 1092365

identity  1092365
fullname  yiplee
user_id   8017d200-7870-4b82-b53f-74bae1d2dad7
mixin_url mixin://users/8017d200-7870-4b82-b53f-74bae1d2dad7
```

### Get Request

Execute HTTP GET Request with path and output the response data

```bash
$ get /network/snapshots limit=1

[
   {
      "amount": "0.0499",
      "asset": {
         "asset_id": "6cfe566e-4aad-470b-8c9a-2fd35b49c68d",
         "asset_key": "eosio.token:EOS",
         "chain_id": "6cfe566e-4aad-470b-8c9a-2fd35b49c68d",
         "icon_url": "https://mixin-images.zeromesh.net/a5dtG-IAg2IO0Zm4HxqJoQjfz-5nf1HWZ0teCyOnReMd3pmB8oEdSAXWvFHt2AJkJj5YgfyceTACjGmXnI-VyRo=s128",
         "name": "EOS",
         "symbol": "EOS",
         "type": "asset"
      },
      "created_at": "2020-02-26T15:46:31.992668Z",
      "snapshot_id": "84be7b16-acfd-4f74-86de-595ec4927b72",
      "source": "TRANSFER_INITIALIZED",
      "type": "snapshot"
   }
]
```

### Post Request

Execute HTTP GET Request with path and output the response data

```bash
$ post /attachments

# with body: post /me full_name=new_name
# with pin:  post /pin/verify --pin

{
   "type": "attachment",
   "attachment_id": "5887806d-e63c-46c4-8f17-d03823b6df8f",
   "upload_url": "https://assets-mixin-one.s3.cn-north-1.amazonaws.com.cn/mixin/attachments/1582732108-c474aa169a065ce2f618be66a426a69c004033c63c8ad58835bcb9799a40c9d5?X-Amz-Algorithm=AWS4-HMAC-SHA256\u0026X-Amz-Credential=AKIAPKOJS6Y7LAXBVZNA%2F20200226%2Fcn-north-1%2Fs3%2Faws4_request\u0026X-Amz-Date=20200226T154828Z\u0026X-Amz-Expires=21600\u0026X-Amz-SignedHeaders=content-type%3Bhost%3Bx-amz-acl\u0026X-Amz-Signature=934c5072c9a5b66533ec5bab5b0280f854426e538a4127685eeeab3fa7af309f",
   "view_url": "https://mixin-assets-cn.zeromesh.net/mixin/attachments/1582732108-c474aa169a065ce2f618be66a426a69c004033c63c8ad58835bcb9799a40c9d5"
}
```

### Upload File

Create an attachment and upload file

```bash
$ upload ~/Downloads/echo.png

# attachment id
8d266870-4679-4464-8789-3d267dfbaa8e
# view url
https://mixin-assets-cn.zeromesh.net/mixin/attachments/1582732253-857c73a883e6b7c352bc8c14879218a01bf5161f19dcebd2738ae0cdccdab8a6
```
