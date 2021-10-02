import Tabs from "@theme/Tabs"
import TabItem from "@theme/TabItem"

# Function Results

The optional `results` subsection contains field definitions for each of the results a
function produces. The layout of the field definitions is identical to that of
the [state](state.md) field definitions.

The schema tool will automatically generate a mutable structure with member variables for
proxies to each result variable in the results map. The user will be able to set the
result variables through this structure, which is passed to the function.

When this subsection is empty or completely omitted, no structure will be generated or
passed to the function.

For example, here is the structure generated for the mutable results for the `getFactor`
function:

<Tabs defaultValue="go"
    values={[
        {label: 'Go', value: 'go'},
        {label: 'Json', value: 'json'},
        {label: 'Rust', value: 'rust'},
    ]}>

<TabItem value="go">
```go
type MutableGetFactorResults struct {
    id int32
}

func (s MutableGetFactorResults) Factor() wasmlib.ScMutableInt64 {
    return wasmlib.NewScMutableInt64(s.id, idxMap[IdxResultFactor])
}
```
</TabItem>
<TabItem value="rust">
```rust
#[derive(Clone, Copy)]
pub struct MutableGetFactorResults {
    pub(crate) id: i32,
}

impl MutableGetFactorResults {
    pub fn factor(&self) -> ScMutableInt64 {
        ScMutableInt64::new(self.id, idx_map(IDX_RESULT_FACTOR))
    }
}
```
</TabItem>
</Tabs>
Note that the schema tool will also generate an immutable version of the structure,
suitable for accessing the results after calling this smart contract function.

In the next section we will look at how so-called thunk functions encapsulate access and
parameter checking and set up the type-safe function-specific contexts.
