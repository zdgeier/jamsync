import {nodeResolve} from "@rollup/plugin-node-resolve"
import typescript from '@rollup/plugin-typescript';

export default {
  input: "./editor.mts",
  output: {
    file: "./editor.bundle.js",
    format: "iife"
  },
  plugins: [nodeResolve(), typescript()]
}