## Function Descriptors

The schema tool provides us with an easy way to access to smart contract functions through
so-called _function descriptors_. These are structures that provide access to the optional
params and results maps through strict compile-time checked interfaces. They will also
allow you to initiate the function by calling it synchronously or posting a request to run
it asynchronously.

The schema tool will generate a specific function descriptor for each func and view. It
will also generate an interface called ScFuncs that can be used to create and initialize
each function descriptor. Here is the code generated for the `dividend` example
in `contract.rs`:

```rust
// @formatter:off

#![allow(dead_code)]

use std::ptr;

use wasmlib::*;

use crate::consts::*;
use crate::params::*;
use crate::results::*;

pub struct DivideCall {
    pub func: ScFunc,
}

pub struct InitCall {
    pub func:   ScFunc,
    pub params: MutableInitParams,
}

pub struct MemberCall {
    pub func:   ScFunc,
    pub params: MutableMemberParams,
}

pub struct SetOwnerCall {
    pub func:   ScFunc,
    pub params: MutableSetOwnerParams,
}

pub struct GetFactorCall {
    pub func:    ScView,
    pub params:  MutableGetFactorParams,
    pub results: ImmutableGetFactorResults,
}

pub struct ScFuncs {
}

impl ScFuncs {
    pub fn divide(_ctx: & dyn ScFuncCallContext) -> DivideCall {
        DivideCall {
            func: ScFunc::new(HSC_NAME, HFUNC_DIVIDE),
        }
    }
    pub fn init(_ctx: & dyn ScFuncCallContext) -> InitCall {
        let mut f = InitCall {
            func:   ScFunc::new(HSC_NAME, HFUNC_INIT),
            params: MutableInitParams { id: 0 },
        };
        f.func.set_ptrs(&mut f.params.id, ptr::null_mut());
        f
    }
    pub fn member(_ctx: & dyn ScFuncCallContext) -> MemberCall {
        let mut f = MemberCall {
            func:   ScFunc::new(HSC_NAME, HFUNC_MEMBER),
            params: MutableMemberParams { id: 0 },
        };
        f.func.set_ptrs(&mut f.params.id, ptr::null_mut());
        f
    }
    pub fn set_owner(_ctx: & dyn ScFuncCallContext) -> SetOwnerCall {
        let mut f = SetOwnerCall {
            func:   ScFunc::new(HSC_NAME, HFUNC_SET_OWNER),
            params: MutableSetOwnerParams { id: 0 },
        };
        f.func.set_ptrs(&mut f.params.id, ptr::null_mut());
        f
    }
    pub fn get_factor(_ctx: & dyn ScViewCallContext) -> GetFactorCall {
        let mut f = GetFactorCall {
            func:    ScView::new(HSC_NAME, HVIEW_GET_FACTOR),
            params:  MutableGetFactorParams { id: 0 },
            results: ImmutableGetFactorResults { id: 0 },
        };
        f.func.set_ptrs(&mut f.params.id, &mut f.results.id);
        f
    }
}

// @formatter:on
```

As you can see a struct has been generated for each of the funcs and views. Note that the
structs only provide access to `params` or `results` when these are specified for the
function. Also note that each struct has a `func` member that can be used to initiate the
function call in certain ways. The `func` member will be of type ScFunc or ScView,
depending on whether the function is a func or a view.

The ScFuncs struct provides a member function for each func or view that will create their
respective function descriptor, initialize it properly, and returns it.

In the next section we will look at how to use function descriptors to call a smart
contract function synchronously.

Next: [Calling Functions](call.md)

