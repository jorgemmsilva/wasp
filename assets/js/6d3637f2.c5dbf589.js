(self.webpackChunkdoc_ops=self.webpackChunkdoc_ops||[]).push([[9352],{3905:function(e,t,n){"use strict";n.d(t,{Zo:function(){return p},kt:function(){return m}});var r=n(7294);function a(e,t,n){return t in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function o(e,t){var n=Object.keys(e);if(Object.getOwnPropertySymbols){var r=Object.getOwnPropertySymbols(e);t&&(r=r.filter((function(t){return Object.getOwnPropertyDescriptor(e,t).enumerable}))),n.push.apply(n,r)}return n}function i(e){for(var t=1;t<arguments.length;t++){var n=null!=arguments[t]?arguments[t]:{};t%2?o(Object(n),!0).forEach((function(t){a(e,t,n[t])})):Object.getOwnPropertyDescriptors?Object.defineProperties(e,Object.getOwnPropertyDescriptors(n)):o(Object(n)).forEach((function(t){Object.defineProperty(e,t,Object.getOwnPropertyDescriptor(n,t))}))}return e}function s(e,t){if(null==e)return{};var n,r,a=function(e,t){if(null==e)return{};var n,r,a={},o=Object.keys(e);for(r=0;r<o.length;r++)n=o[r],t.indexOf(n)>=0||(a[n]=e[n]);return a}(e,t);if(Object.getOwnPropertySymbols){var o=Object.getOwnPropertySymbols(e);for(r=0;r<o.length;r++)n=o[r],t.indexOf(n)>=0||Object.prototype.propertyIsEnumerable.call(e,n)&&(a[n]=e[n])}return a}var l=r.createContext({}),c=function(e){var t=r.useContext(l),n=t;return e&&(n="function"==typeof e?e(t):i(i({},t),e)),n},p=function(e){var t=c(e.components);return r.createElement(l.Provider,{value:t},e.children)},u={inlineCode:"code",wrapper:function(e){var t=e.children;return r.createElement(r.Fragment,{},t)}},d=r.forwardRef((function(e,t){var n=e.components,a=e.mdxType,o=e.originalType,l=e.parentName,p=s(e,["components","mdxType","originalType","parentName"]),d=c(n),m=a,f=d["".concat(l,".").concat(m)]||d[m]||u[m]||o;return n?r.createElement(f,i(i({ref:t},p),{},{components:n})):r.createElement(f,i({ref:t},p))}));function m(e,t){var n=arguments,a=t&&t.mdxType;if("string"==typeof e||a){var o=n.length,i=new Array(o);i[0]=d;var s={};for(var l in t)hasOwnProperty.call(t,l)&&(s[l]=t[l]);s.originalType=e,s.mdxType="string"==typeof e?e:a,i[1]=s;for(var c=2;c<o;c++)i[c]=n[c];return r.createElement.apply(null,i)}return r.createElement.apply(null,n)}d.displayName="MDXCreateElement"},2897:function(e,t,n){"use strict";n.r(t),n.d(t,{frontMatter:function(){return s},contentTitle:function(){return l},metadata:function(){return c},toc:function(){return p},default:function(){return d}});var r=n(2122),a=n(9756),o=(n(7294),n(3905)),i=["components"],s={},l="Invoking smart contracts. Calling a view",c={unversionedId:"tutorial/07",id:"tutorial/07",isDocsHomePage:!1,title:"Invoking smart contracts. Calling a view",description:"The statement in the above example calls the view entry point getString of the",source:"@site/docs/tutorial/07.md",sourceDirName:"tutorial",slug:"/tutorial/07",permalink:"/docs/tutorial/07",editUrl:"https://github.com/iotaledger/chronicle.rs/tree/main/docs/docs/tutorial/07.md",version:"current",frontMatter:{},sidebar:"tutorialSidebar",previous:{title:"Invoking smart contracts. Sending a request `on-ledger`",permalink:"/docs/tutorial/06"},next:{title:"Accounts: deposit and withdraw tokens",permalink:"/docs/tutorial/08"}},p=[{value:"Decoding results returned by <em>PostRequestSync</em> and <em>CallView</em>",id:"decoding-results-returned-by-postrequestsync-and-callview",children:[]}],u={toc:p};function d(e){var t=e.components,s=(0,a.Z)(e,i);return(0,o.kt)("wrapper",(0,r.Z)({},u,s,{components:t,mdxType:"MDXLayout"}),(0,o.kt)("h1",{id:"invoking-smart-contracts-calling-a-view"},"Invoking smart contracts. Calling a view"),(0,o.kt)("p",null,"The statement in the above example calls the view entry point ",(0,o.kt)("inlineCode",{parentName:"p"},"getString")," of the\nsmart contract ",(0,o.kt)("inlineCode",{parentName:"p"},"example1")," without parameters:"),(0,o.kt)("pre",null,(0,o.kt)("code",{parentName:"pre",className:"language-go"},'res, err := chain.CallView("example1", "getString")\n')),(0,o.kt)("p",null,"The call returns both a collection of key/value pairs ",(0,o.kt)("inlineCode",{parentName:"p"},"res")," and an error result\n",(0,o.kt)("inlineCode",{parentName:"p"},"err")," in the typical Go fashion."),(0,o.kt)("p",null,(0,o.kt)("img",{src:n(9663).Z})),(0,o.kt)("p",null,"The basic principle of calling a view is similar to sending a request to the\nsmart contract. The essential difference is that calling a view does not\nconstitute an asynchronous transaction, it is just a direct synchronous\ncall to the view entry point function, exposed by the smart contract."),(0,o.kt)("p",null,"Therefore, calling a view doesn't involve any token transfers. Sending a\nrequest (a transaction) to a view entry point will result in an exception. It\nwill return all attached tokens back to the sender (minus fees, if any)."),(0,o.kt)("p",null,"Views are used to retrieve information about the state of the smart contract,\nfor example to display the information on a website. Certain ",(0,o.kt)("em",{parentName:"p"},"Solo")," methods such\nas ",(0,o.kt)("inlineCode",{parentName:"p"},"chain.GetInfo"),", ",(0,o.kt)("inlineCode",{parentName:"p"},"chain.GetFeeInfo")," and ",(0,o.kt)("inlineCode",{parentName:"p"},"chain.GetTotalAssets")," call views of\nthe core smart contracts behind scenes to retrieve the information about the\nchain or a specific smart contract."),(0,o.kt)("h3",{id:"decoding-results-returned-by-postrequestsync-and-callview"},"Decoding results returned by ",(0,o.kt)("em",{parentName:"h3"},"PostRequestSync")," and ",(0,o.kt)("em",{parentName:"h3"},"CallView")),(0,o.kt)("p",null,"The following is a specific technicality of the Go environment of ",(0,o.kt)("em",{parentName:"p"},"Solo"),"."),(0,o.kt)("p",null,"The result returned by the call to an entry point from the ",(0,o.kt)("em",{parentName:"p"},"Solo")," environment\nis in the form of key/value pairs, the ",(0,o.kt)("inlineCode",{parentName:"p"},"dict.Dict")," type. It is an alias of ",(0,o.kt)("inlineCode",{parentName:"p"},"map[string][]byte"),".\nThe ",(0,o.kt)("a",{parentName:"p",href:"https://github.com/iotaledger/wasp/blob/master/packages/kv/dict/dict.go"},"dict.Dict"),"\npackage implements the ",(0,o.kt)("inlineCode",{parentName:"p"},"kv.KVStore")," interface and provides a lot of useful\nfunctions to handle this form of key/value storage."),(0,o.kt)("p",null,"In normal operation of smart contracts one can only retrieve results returned by\nview calls, since view calls are synchronous. Sending a request to a smart\ncontract is normally an asynchronous operation, and the caller cannot retrieve\nthe result. However, in the ",(0,o.kt)("em",{parentName:"p"},"Solo")," environment, the call to ",(0,o.kt)("inlineCode",{parentName:"p"},"PostRequestSync")," is\nsynchronous, and the caller can inspect the result: this is a convenient\ndifference between the mocked ",(0,o.kt)("em",{parentName:"p"},"Solo")," environment, and the distributed UTXO\nLedger used by Wasp nodes. It can be used to make assertions about the results\nof a call in the tests."),(0,o.kt)("p",null,"In the ",(0,o.kt)("inlineCode",{parentName:"p"},"TestTutorial3")," example ",(0,o.kt)("inlineCode",{parentName:"p"},"res")," is a dictionary where keys and values are binary slices.\nThe fragment"),(0,o.kt)("pre",null,(0,o.kt)("code",{parentName:"pre",className:"language-go"},'    par := kvdecoder.New(res, chain.Log)\n    returnedString := par.MustGetString("paramString")\n')),(0,o.kt)("p",null,"creates object ",(0,o.kt)("inlineCode",{parentName:"p"},"par")," which offer all kinds of usefull function to take and decode key/value pairs in the ",(0,o.kt)("inlineCode",{parentName:"p"},"kv.KVStoreReader"),"\ninterface (the logger is used to log the data conversion and other errors when occur).",(0,o.kt)("br",{parentName:"p"}),"\n","The second statement takes the value of the key ",(0,o.kt)("inlineCode",{parentName:"p"},"paramString")," from the key value store and attempts\nto decode it as a ",(0,o.kt)("inlineCode",{parentName:"p"},"string"),". It panics of key dos not exists or data conversion fails."))}d.isMDXComponent=!0},9663:function(e,t,n){"use strict";t.Z=n.p+"assets/images/call_view-d3c697d7600a2a207d399c7dddf4b224.png"}}]);