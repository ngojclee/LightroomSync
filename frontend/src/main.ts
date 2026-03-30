import { mountApp } from "./App";
import "./styles.css";

const root = document.getElementById("app");
if (!root) {
  throw new Error("App root not found");
}

mountApp(root);
