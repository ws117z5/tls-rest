export { default as DataPage } from "./DataPage";
// App pages (live under src/pages/*). They must be exported here so Config's
// barrel can match them to their backend menu entry by href; otherwise their
// route isn't registered and the menu link falls through to Home.
export { default as PapersPage } from "../../pages/papers/Papers";
export { default as NetworkMapperPage } from "../../pages/networkmapper/NetworkMapper";
export { default as OpenCVPage } from "../../pages/opencv/OpenCV";