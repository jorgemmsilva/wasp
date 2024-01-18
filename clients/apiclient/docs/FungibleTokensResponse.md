# FungibleTokensResponse

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**BaseTokens** | **string** | The base tokens (uint64 as string) | 
**NativeTokens** | **map[string]string** |  | 

## Methods

### NewFungibleTokensResponse

`func NewFungibleTokensResponse(baseTokens string, nativeTokens map[string]string, ) *FungibleTokensResponse`

NewFungibleTokensResponse instantiates a new FungibleTokensResponse object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewFungibleTokensResponseWithDefaults

`func NewFungibleTokensResponseWithDefaults() *FungibleTokensResponse`

NewFungibleTokensResponseWithDefaults instantiates a new FungibleTokensResponse object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetBaseTokens

`func (o *FungibleTokensResponse) GetBaseTokens() string`

GetBaseTokens returns the BaseTokens field if non-nil, zero value otherwise.

### GetBaseTokensOk

`func (o *FungibleTokensResponse) GetBaseTokensOk() (*string, bool)`

GetBaseTokensOk returns a tuple with the BaseTokens field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetBaseTokens

`func (o *FungibleTokensResponse) SetBaseTokens(v string)`

SetBaseTokens sets BaseTokens field to given value.


### GetNativeTokens

`func (o *FungibleTokensResponse) GetNativeTokens() map[string]string`

GetNativeTokens returns the NativeTokens field if non-nil, zero value otherwise.

### GetNativeTokensOk

`func (o *FungibleTokensResponse) GetNativeTokensOk() (*map[string]string, bool)`

GetNativeTokensOk returns a tuple with the NativeTokens field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetNativeTokens

`func (o *FungibleTokensResponse) SetNativeTokens(v map[string]string)`

SetNativeTokens sets NativeTokens field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


