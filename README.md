# mixin-cli
Command-line applications to manage mixin dapps

## Install

### From Source Code

```bash
$ git clone git@github.com:fox-one/mixin-cli.git
$ cd mixin-cli
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

There are three ways to specify the keystore:

1. Use the ```--file``` option to specify the keystore file path.
2. Use ```--stdin``` option to read keystore content from os.Stdin.
3. Use the name of keystore file in ```~/.mixin-cli```, for example, ```mixin-cli bot``` will use ~/.mixin-cli/bot.json.
4. Use the ```--pin``` to specify the pin code.

## Commands

### List assets with balance

```bash
$ mixin-cli asset list

AssetId                               Symbol  Name         Balance
965e5c6e-434c-3fa9-b780-c50f43cd955c  CNB     Chui Niu Bi  9998898.552
Total USD Value: 0.09998898552
```

### Search asset with asset id or symbol

search by asset id:
```bash
$ mixin-cli asset search 965e5c6e-434c-3fa9-b780-c50f43cd955c

AssetId   965e5c6e-434c-3fa9-b780-c50f43cd955c
Symbol    CNB
Name      Chui Niu Bi
ChainId   43d61dcd-e413-450d-80b8-101d5e903357
PriceUsd  0.00000001
IconUrl   https://mixin-images.zeromesh.net/0sQY63dDMkWTURkJVjowWY6Le4ICjAFuu3ANVyZA4uI3UdkbuOT5fjJUT82ArNYmZvVcxDXyNjxoOv0TAYbQTNKS=s128
```

search by asset symbol:
```bash
$ mixin-cli asset search BOX

AssetId                               Symbol  Name       ChainId                               PriceUsd
f5ef6b5d-cc5a-3d90-b2c0-a2fd386e7a3c  BOX     BOX Token  43d61dcd-e413-450d-80b8-101d5e903357  7.2782709
2fea3c35-7fb7-3e01-91b1-99b3c744a729  BOX     BOX Token  43d61dcd-e413-450d-80b8-101d5e903357  0
20b8c101-dffa-31c9-bf6e-d93a086686af  BOX     BOX Token  43d61dcd-e413-450d-80b8-101d5e903357  0
```

### Custom http request of mixin api

get request with query:
```bash
# GET /users/25566?foo=bar
$ mixin-cli http /users/25566 foo==bar

{
  "type": "user",
  "user_id": "fcb87491-4fa0-4c2f-b387-262b63cbc112",
  "identity_number": "25566",
  "phone": "",
  "full_name": "人",
  "biography": "Send me any transfer to start a conversation 💰",
  "avatar_url": "https://mixin-images.zeromesh.net/MiGX1hgHm7cpLznNYlaxgPTcj8LisYjAUUwcmOrZcwBgIZqaAUSfeuirJ2hAcZES9y3T6dDy31ljbbD2dpJHaHFgn_kkXlAZm_o=s256",
  "relationship": "FRIEND",
  "mute_until": "2020-05-25T08:23:09.409520437Z",
  "created_at": "2017-11-27T02:27:58.398423112Z",
  "is_verified": false,
  "is_scam": false
}
```

post request with simple body:
```bash
$ mixin-cli http post /attachments number:=1 foo=bar

{
  "type": "attachment",
  "attachment_id": "a3bde58a-4861-418b-860d-aa26a001ac7b",
  "upload_url": "https://moments-shou-tv.s3.amazonaws.com/mixin/attachments/1638364433-4c67c4840fa610cb2570702a76c03fc79d46f29a8947bd24264fa624f2d51543?X-Amz-Algorithm=AWS4-HMAC-SHA256\u0026X-Amz-Credential=AKIAJW6D5Q3Z5WYA2KRQ%2F20211201%2Fus-east-1%2Fs3%2Faws4_request\u0026X-Amz-Date=20211201T131353Z\u0026X-Amz-Expires=21600\u0026X-Amz-SignedHeaders=content-type%3Bhost%3Bx-amz-acl\u0026X-Amz-Signature=90d8e950f17b87af94010d24e6a35f89c5325d6373afe5cfc7fe451eb32babd2",
  "view_url": "https://mixin-assets.zeromesh.net/mixin/attachments/1638364433-4c67c4840fa610cb2570702a76c03fc79d46f29a8947bd24264fa624f2d51543",
  "created_at": "2021-12-01T13:13:53.756443264Z"
}
```

post request with raw json body
```bash
$ mixin-cli http post /attachments --raw '{"foo":"bar"}'

{
  "type": "attachment",
  "attachment_id": "db3e616e-8e91-4e08-b75c-890ad579649e",
  "upload_url": "https://moments-shou-tv.s3.amazonaws.com/mixin/attachments/1638364537-9b603a75c11768e1193401048fcf5ae5f01fff97ef7eeca40ecc0908407e7788?X-Amz-Algorithm=AWS4-HMAC-SHA256\u0026X-Amz-Credential=AKIAJW6D5Q3Z5WYA2KRQ%2F20211201%2Fus-east-1%2Fs3%2Faws4_request\u0026X-Amz-Date=20211201T131537Z\u0026X-Amz-Expires=21600\u0026X-Amz-SignedHeaders=content-type%3Bhost%3Bx-amz-acl\u0026X-Amz-Signature=4458a68e361c72f0ba17de9902ca4eddf407ea6dd89512c9bd7008100ae4ccc0",
  "view_url": "https://mixin-assets.zeromesh.net/mixin/attachments/1638364537-9b603a75c11768e1193401048fcf5ae5f01fff97ef7eeca40ecc0908407e7788",
  "created_at": "2021-12-01T13:15:37.515308839Z"
}
```

### Generate mixin auth token with custom path & expire duration

```bash
$ mixin-cli sign /fiats --exp 262800h

sign GET /fiats with request id b1785af1-6974-4cbe-a8e0-4b1f6d4680ea & exp 262800h0m0s

eyJhbGciOiJSUzUxMiIsInR5cCI6IkpXVCJ9.eyJleHAiOjI1ODQ0NDQ2MzQsImlhdCI6MTYzODM2NDYzNCwianRpIjoiYjE3ODVhZjEtNjk3NC00Y2JlLWE4ZTAtNGIxZjZkNDY4MGVhIiwic2NwIjoiRlVMTCIsInNpZCI6ImRiMmYzMmJiLWYyYTUtNDJiMS1iOTQ2LTYzYTRlMTI5YjAyYyIsInNpZyI6ImNjNWY5ZjNlYjVmYTdlZWQ3NzlmZGYyMWQwNmY5MTQzYmNhMjNhNjEyZTE0M2E1YzFjZjhmZDQzNWFmNmEwYzEiLCJ1aWQiOiI1YzRmMzBhNi0xZjQ5LTQzYzMtYjM3Yi1jMDFhYWU1MTkxYWYifQ.LEQqG4Ae0I1ep-fabSGDM-ITfKrWjX21gTdQvFpnpAfbn_0N_m7vN3JIy453TMVKn6s7S0ngAYBfS0SuFxSvVPMYOMyvUnKadNVxgCWP9wu2DLRa_dzzJGzjLuvJStNnl7xk6HlKdRxFQ5xvGXe6MLijeaOOUuo6Sr2ceZ7tprk
```

### Transfer to any opponent

```
$ mixin-cli transfer --asset 965e5c6e-434c-3fa9-b780-c50f43cd955c \
 --amount 100 \
 --opponent 8017d200-7870-4b82-b53f-74bae1d2dad7 \
 --memo hahaha

{
  "snapshot_id": "fbc06508-1d8d-49cb-b17b-af7c1532e06c",
  "created_at": "2021-12-01T13:20:34.81424Z",
  "trace_id": "6e0e4349-8f30-4f18-96da-0f0264cf3149",
  "asset_id": "965e5c6e-434c-3fa9-b780-c50f43cd955c",
  "opponent_id": "8017d200-7870-4b82-b53f-74bae1d2dad7",
  "amount": "-100",
  "opening_balance": "9998898.552",
  "closing_balance": "9998798.552",
  "memo": "hahaha",
  "type": "transfer"
}
```

### Transfer to a multisig group

```bash
$ mixin-cli transfer --asset 965e5c6e-434c-3fa9-b780-c50f43cd955c \
--amount 100 \
--receivers 8017d200-7870-4b82-b53f-74bae1d2dad7 \
--receivers 170e40f0-627f-4af2-acf5-0f25c009e523 \
--threshold 2 \
--memo hahaha

{
  "type": "raw",
  "snapshot": "",
  "opponent_key": "",
  "asset_id": "965e5c6e-434c-3fa9-b780-c50f43cd955c",
  "amount": "-100",
  "trace_id": "917ec61f-d703-472f-afd4-6f32c99ea8af",
  "memo": "hahaha",
  "state": "signed",
  "created_at": "1970-01-01T00:03:39+00:03",
  "transaction_hash": "941bd691338f8077cfe7edb53a0315c0299e514921f1af9964828629f413ee95",
  "snapshot_at": "0001-01-01T00:00:00Z"
}
```

### Upload a file as attachment

```bash
$ mixin-cli upload ~/path/to/the/file

# attachment id
9b490939-9daf-4f09-8296-54f995f143d7
https://mixin-assets.zeromesh.net/mixin/attachments/1638365395-614893646cab4f829ac6936ea57345bc27e6751f36e22b06dbbbd9df30c7c754
```

### Create a new user

```bash
$ mixin-cli user create haha --pin 123456

{
  "client_id": "041d9a17-fb5d-33fb-9efc-182dcc68b58f",
  "session_id": "b6a3430c-0b00-4bba-9f8f-555d0ef3c9c2",
  "private_key": "DTev80bjzTas1kkSxYH34jQcjVc2FviMTaf3KrDtPIptz8...",
  "pin_token": "BcuNLJHM5OdZ6UPcSaxNgsP8HHU873nJjlB+CEKFims=",
  "scope": "",
  "pin": "123456"
}
```

### Show own profile

```bash
$ mixin-cli user me

{
  "user_id": "5c4f30a6-1f49-43c3-b37b-c01aae5191af",
  "identity_number": "7000101692",
  "phone": "5c4f30a6-1f49-43c3-b37b-c01aae5191af",
  "full_name": "echo",
  "biography": "我是群消息通知机器人 echo。\r\n群主拉我进群，开始使用吧！",
  "avatar_url": "https://mixin-images.zeromesh.net/kQ4h_g2V8VRcl4DjqAhWJcthV4yEXl8Ytjrc8fx777LIA3ernaxU7UqcFolYKvWXJOtY7pkMG8NvKCtAhEJM3ptW=s256",
  "relationship": "ME",
  "mute_until": "0001-01-01T00:00:00Z",
  "created_at": "2018-12-23T14:20:34.188140494Z",
  "session_id": "db2f32bb-f2a5-42b1-b946-63a4e129b02c",
  "code_id": "7d97440b-fb4f-4ddd-a74b-58d8806835e0",
  "code_url": "https://mixin.one/codes/7d97440b-fb4f-4ddd-a74b-58d8806835e0",
  "has_pin": true,
  "receive_message_source": "EVERYBODY",
  "accept_conversation_source": "EVERYBODY",
  "accept_search_source": "EVERYBODY",
  "fiat_currency": "USD",
  "app": {
    "updated_at": "2021-08-08T17:05:19.73808177Z",
    "app_id": "5c4f30a6-1f49-43c3-b37b-c01aae5191af",
    "app_number": "7000101692",
    "redirect_uri": "https://ocean.one/auth",
    "home_uri": "https://workflow.yiplee.com",
    "name": "echo",
    "icon_url": "https://mixin-images.zeromesh.net/6EoTjFGVMyPQJOz3JaCkGssmPbwLZviBEmLqmgXqLITQW_Q3DWiOAjmEHvGk8R53qebinHePo1Dq4ngTSD5fRw=s256",
    "description": "我是群消息通知机器人 echo。\r\n群主拉我进群，开始使用吧！",
    "capabilities": [
      "GROUP",
      "IMMERSIVE",
      "CONTACT"
    ],
    "resource_patterns": [],
    "category": "TOOLS",
    "creator_id": "8017d200-7870-4b82-b53f-74bae1d2dad7"
  }
}
```

### Search user by mixin id or identity number

```bash
$ mixin-cli echo user search 25566

{
  "user_id": "fcb87491-4fa0-4c2f-b387-262b63cbc112",
  "identity_number": "25566",
  "full_name": "人",
  "biography": "Send me any transfer to start a conversation 💰",
  "avatar_url": "https://mixin-images.zeromesh.net/MiGX1hgHm7cpLznNYlaxgPTcj8LisYjAUUwcmOrZcwBgIZqaAUSfeuirJ2hAcZES9y3T6dDy31ljbbD2dpJHaHFgn_kkXlAZm_o=s256",
  "relationship": "FRIEND",
  "mute_until": "2020-05-25T08:23:09.409520437Z",
  "created_at": "2017-11-27T02:27:58.398423112Z"
}
```

