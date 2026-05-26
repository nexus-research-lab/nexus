/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL?: string;
  readonly VITE_WS_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

declare module "x-data-spreadsheet/dist/xspreadsheet.js" {
  import Spreadsheet from "x-data-spreadsheet";

  export default Spreadsheet;
}
