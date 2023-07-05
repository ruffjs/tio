# tio/web

## About Web Page

Also named TIO Playground, as a graphic ui pwa, which is use to understand more about tio's designing and apis for developers.

It provides a query view for adding things to tio, and querying things registered in tio.

And, it also provides detail views of each thing, for developers to operate their things, such as checking shadow, set tags, set desires and report like a device. And all of these operations, developers could see the operating detail.

In addition, it provides a multiple-client-mqtt-tool, for developers connect to tio as things or as servers.

## Requires

Since it is writen by Vue 3 and TypeScript in Vite, you should prepare environments listing below before run it:

- Node: version >= 16.13.1

- Yarn: version >= 1.22.17

- Browser: Chrome or Edge. The newer version, the better experience.

more about Vue 3, check out the [script setup docs](https://v3.vuejs.org/api/sfc-script-setup.html#sfc-script-setup).

## Recommended IDE Setup

- [VS Code](https://code.visualstudio.com/)

- [Volar](https://marketplace.visualstudio.com/items?itemName=Vue.volar) (and disable Vetur)

- [TypeScript Vue Plugin (Volar)](https://marketplace.visualstudio.com/items?itemName=Vue.vscode-typescript-vue-plugin).

## Type Support For `.vue` Imports in TS

TypeScript cannot handle type information for `.vue` imports by default, so we replace the `tsc` CLI with `vue-tsc` for type checking. In editors, we need [TypeScript Vue Plugin (Volar)](https://marketplace.visualstudio.com/items?itemName=Vue.vscode-typescript-vue-plugin) to make the TypeScript language service aware of `.vue` types.

If the standalone TypeScript plugin doesn't feel fast enough to you, Volar has also implemented a [Take Over Mode](https://github.com/johnsoncodehk/volar/discussions/471#discussioncomment-1361669) that is more performant. You can enable it by the following steps:

1. Disable the built-in TypeScript Extension
   1. Run `Extensions: Show Built-in Extensions` from VSCode's command palette
   2. Find `TypeScript and JavaScript Language Features`, right click and select `Disable (Workspace)`
2. Reload the VSCode window by running `Developer: Reload Window` from the command palette.

## Let's Play

### For Web Development

Modify file web/vite.config.ts, make api proxy of dev server direct to a running tio.

exec this command:

```bash
# cd web
yarn && yarn dev
```

then open this url:

http://localhost:3333/web/

### For Intergrated in TIO Development

Make sure your go env works at first,

exec this command:

```bash
# cd web
yarn && yarn build
```

then run your tio:

```bash
cd .. && go run cmd/tio/main.go
```
