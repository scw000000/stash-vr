# deovr-json-schema

A [JSON-Schema](https://json-schema.org/) for [DeoVr json](https://deovr.com/app/doc)

## JSON Usage

```json
{
  "$schema": "https://unpkg.com/deovr-json-schema/dist/schema.json",
  "scenes": []
}
```

## Typescript Usage

```sh
# npm
npm install --save-dev deovr-json-schema
# yarn
yarn add --dev deovr-json-schema
```

```ts
import { DeoVrJson } from "deovr-json-schema";

const json: DeoVrJson = {
  scenes: [],
};
```
