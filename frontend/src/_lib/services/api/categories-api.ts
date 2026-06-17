import type { CategorySettings } from "@/_lib/types/setting-types";
import { request } from "./api";

/** Fetch the current category rules + master toggle. */
export async function getCategories(): Promise<CategorySettings> {
  return request<CategorySettings>(`/api/v1/settings/categories`);
}

/** Replace the category rules + master toggle. */
export async function putCategories(
  data: CategorySettings,
): Promise<CategorySettings> {
  return request<CategorySettings>(`/api/v1/settings/categories`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}
