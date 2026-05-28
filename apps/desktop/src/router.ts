import { createRouter, createMemoryHistory, type RouteRecordRaw } from "vue-router";
import FolderListView from "./views/FolderListView.vue";
import ProjectDashboard from "./views/ProjectDashboard.vue";
import IgnoreEditor from "./views/IgnoreEditor.vue";
import MaskManager from "./views/MaskManager.vue";

const routes: RouteRecordRaw[] = [
  { path: "/", component: FolderListView },
  { path: "/folders/:id", component: ProjectDashboard, props: true },
  { path: "/folders/:id/ignore", component: IgnoreEditor, props: true },
  { path: "/folders/:id/masks", component: MaskManager, props: true },
];

export const router = createRouter({
  history: createMemoryHistory(),
  routes,
});
