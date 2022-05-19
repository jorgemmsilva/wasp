// COPYRIGHT OF A TEST SCHEMA DEFINITION 1
// COPYRIGHT OF A TEST SCHEMA DEFINITION 2

// (Re-)generated by schema tool
// >>>> DO NOT CHANGE THIS FILE! <<<<
// Change the json schema instead

import * as wasmtypes from "wasmlib/wasmtypes";
import * as sc from "./index";

export class ImmutableTestFunc1Results extends wasmtypes.ScProxy {
	// comment for length
	length(): wasmtypes.ScImmutableUint32 {
		return new wasmtypes.ScImmutableUint32(this.proxy.root(sc.ResultLength));
	}
}

export class MutableTestFunc1Results extends wasmtypes.ScProxy {
	// comment for length
	length(): wasmtypes.ScMutableUint32 {
		return new wasmtypes.ScMutableUint32(this.proxy.root(sc.ResultLength));
	}
}

export class ImmutableTestView1Results extends wasmtypes.ScProxy {
	// comment for length
	length(): wasmtypes.ScImmutableUint32 {
		return new wasmtypes.ScImmutableUint32(this.proxy.root(sc.ResultLength));
	}
}

export class MutableTestView1Results extends wasmtypes.ScProxy {
	// comment for length
	length(): wasmtypes.ScMutableUint32 {
		return new wasmtypes.ScMutableUint32(this.proxy.root(sc.ResultLength));
	}
}
