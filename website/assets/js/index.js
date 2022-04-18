import { versionWarning } from "./versionWarning";
import {OfflineSearch} from "./offlineSearch";

// show warning when viewing outdated or preview docs
document.addEventListener('DOMContentLoaded', versionWarning, false);
// enable offine search
document.addEventListener('DOMContentLoaded', () => new OfflineSearch(), false);
