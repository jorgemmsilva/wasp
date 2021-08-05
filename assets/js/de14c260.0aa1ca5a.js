(self.webpackChunkdoc_ops=self.webpackChunkdoc_ops||[]).push([[2941],{3905:function(e,t,n){"use strict";n.d(t,{Zo:function(){return s},kt:function(){return m}});var r=n(7294);function o(e,t,n){return t in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function i(e,t){var n=Object.keys(e);if(Object.getOwnPropertySymbols){var r=Object.getOwnPropertySymbols(e);t&&(r=r.filter((function(t){return Object.getOwnPropertyDescriptor(e,t).enumerable}))),n.push.apply(n,r)}return n}function a(e){for(var t=1;t<arguments.length;t++){var n=null!=arguments[t]?arguments[t]:{};t%2?i(Object(n),!0).forEach((function(t){o(e,t,n[t])})):Object.getOwnPropertyDescriptors?Object.defineProperties(e,Object.getOwnPropertyDescriptors(n)):i(Object(n)).forEach((function(t){Object.defineProperty(e,t,Object.getOwnPropertyDescriptor(n,t))}))}return e}function c(e,t){if(null==e)return{};var n,r,o=function(e,t){if(null==e)return{};var n,r,o={},i=Object.keys(e);for(r=0;r<i.length;r++)n=i[r],t.indexOf(n)>=0||(o[n]=e[n]);return o}(e,t);if(Object.getOwnPropertySymbols){var i=Object.getOwnPropertySymbols(e);for(r=0;r<i.length;r++)n=i[r],t.indexOf(n)>=0||Object.prototype.propertyIsEnumerable.call(e,n)&&(o[n]=e[n])}return o}var l=r.createContext({}),p=function(e){var t=r.useContext(l),n=t;return e&&(n="function"==typeof e?e(t):a(a({},t),e)),n},s=function(e){var t=p(e.components);return r.createElement(l.Provider,{value:t},e.children)},u={inlineCode:"code",wrapper:function(e){var t=e.children;return r.createElement(r.Fragment,{},t)}},d=r.forwardRef((function(e,t){var n=e.components,o=e.mdxType,i=e.originalType,l=e.parentName,s=c(e,["components","mdxType","originalType","parentName"]),d=p(n),m=o,f=d["".concat(l,".").concat(m)]||d[m]||u[m]||i;return n?r.createElement(f,a(a({ref:t},s),{},{components:n})):r.createElement(f,a({ref:t},s))}));function m(e,t){var n=arguments,o=t&&t.mdxType;if("string"==typeof e||o){var i=n.length,a=new Array(i);a[0]=d;var c={};for(var l in t)hasOwnProperty.call(t,l)&&(c[l]=t[l]);c.originalType=e,c.mdxType="string"==typeof e?e:o,a[1]=c;for(var p=2;p<i;p++)a[p]=n[p];return r.createElement.apply(null,a)}return r.createElement.apply(null,n)}d.displayName="MDXCreateElement"},3003:function(e,t,n){"use strict";n.r(t),n.d(t,{frontMatter:function(){return c},contentTitle:function(){return l},metadata:function(){return p},toc:function(){return s},default:function(){return d}});var r=n(2122),o=n(9756),i=(n(7294),n(3905)),a=["components"],c={},l="Docker",p={unversionedId:"misc/docker",id:"misc/docker",isDocsHomePage:!1,title:"Docker",description:"This page describes the configuration of the Wasp node in combination with Docker.",source:"@site/docs/misc/docker.md",sourceDirName:"misc",slug:"/misc/docker",permalink:"/docs/misc/docker",editUrl:"https://github.com/iotaledger/chronicle.rs/tree/main/docs/docs/misc/docker.md",version:"current",frontMatter:{}},s=[{value:"Introduction",id:"introduction",children:[]},{value:"Running a Wasp node",id:"running-a-wasp-node",children:[{value:"Configuration",id:"configuration",children:[]}]}],u={toc:s};function d(e){var t=e.components,n=(0,o.Z)(e,a);return(0,i.kt)("wrapper",(0,r.Z)({},u,n,{components:t,mdxType:"MDXLayout"}),(0,i.kt)("h1",{id:"docker"},"Docker"),(0,i.kt)("p",null,"This page describes the configuration of the Wasp node in combination with Docker."),(0,i.kt)("h2",{id:"introduction"},"Introduction"),(0,i.kt)("p",null,"The dockerfile is separated into several stages which effectively splits Wasp into four small pieces:"),(0,i.kt)("ul",null,(0,i.kt)("li",{parentName:"ul"},"Testing",(0,i.kt)("ul",{parentName:"li"},(0,i.kt)("li",{parentName:"ul"},"Unit testing"),(0,i.kt)("li",{parentName:"ul"},"Integration testing"))),(0,i.kt)("li",{parentName:"ul"},"Wasp CLI"),(0,i.kt)("li",{parentName:"ul"},"Wasp Node")),(0,i.kt)("h2",{id:"running-a-wasp-node"},"Running a Wasp node"),(0,i.kt)("p",null,"Checkout the project, switch to 'develop' and build the main image:"),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},"$ git clone -b develop https://github.com/iotaledger/wasp.git\n$ cd wasp\n$ docker build -t wasp-node .\n")),(0,i.kt)("p",null,"The build process will copy the docker_config.json file into the image which will use it when the node gets started. "),(0,i.kt)("p",null,"By default, the build process will use ",(0,i.kt)("inlineCode",{parentName:"p"},"-tags rocksdb")," as a build argument. This argument can be modified with ",(0,i.kt)("inlineCode",{parentName:"p"},"--build-arg BUILD_TAGS=<tags>"),"."),(0,i.kt)("p",null,"Depending on the use case, Wasp requires a different GoShimmer hostname which can be changed at this part inside the docker_config.json file: "),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},'  "nodeconn": {\n    "address": "goshimmer:5000"\n  },\n')),(0,i.kt)("p",null,"The Wasp node can be started like so:"),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},"$ docker run wasp-node\n")),(0,i.kt)("h3",{id:"configuration"},"Configuration"),(0,i.kt)("p",null,"After the build process has been completed, it is still possible to inject a different configuration file into a new container. "),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},"$ docker run -v $(pwd)/alternative_docker_config.json:/run/config.json wasp-node\n")),(0,i.kt)("p",null,"Further configuration is possible using arguments:"),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},"$ docker run wasp-node --nodeconn.address=alt_goshimmer:5000 \n")),(0,i.kt)("p",null,"To get a list of all available arguments, run the node with the argument '--help'"),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},"$ docker run wasp-node --help\n")),(0,i.kt)("h1",{id:"wasp-cli"},"Wasp CLI"),(0,i.kt)("p",null,"It is possible to create a micro image that just contains the wasp-cli application without any Wasp node related additions."),(0,i.kt)("p",null,"This might be helpful if it's required to control but not to run a Wasp node."),(0,i.kt)("p",null,"The image can be created like this:"),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},"$docker build --target wasp-cli -t wasp-cli . \n")),(0,i.kt)("p",null,"Like with the Wasp node setup, the container gets started by:"),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},"$ docker run wasp-cli\n")),(0,i.kt)("p",null,"and can be controlled with further arguments:"),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},"$ docker run wasp-cli --help\n")),(0,i.kt)("h1",{id:"testing"},"Testing"),(0,i.kt)("p",null,"Wip"))}d.isMDXComponent=!0}}]);