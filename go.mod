module github.com/fox-one/mixin-cli

go 1.13

replace github.com/fox-one/mixin-sdk-go => ../mixin-sdk-go

require (
	github.com/andrew-d/go-termutil v0.0.0-20150726205930-009166a695a2
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d
	github.com/btcsuite/btcutil v1.0.3-0.20211129182920-9c4bbabe7acd // indirect
	github.com/fox-one/mixin-sdk-go v1.6.14
	github.com/fox-one/pkg v1.5.5
	github.com/gofrs/uuid v4.3.0+incompatible
	github.com/iancoleman/strcase v0.2.0
	github.com/itchyny/gojq v0.12.5
	github.com/json-iterator/go v1.1.12
	github.com/manifoldco/promptui v0.8.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/nojima/httpie-go v0.7.0
	github.com/ryanuber/columnize v2.1.2+incompatible
	github.com/shopspring/decimal v1.3.1
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.1
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e // indirect
	golang.org/x/net v0.0.0-20220615171555-694bf12d69de // indirect
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f // indirect
	golang.org/x/term v0.2.0
	google.golang.org/protobuf v1.28.0 // indirect
)
