import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  getExtensionToken,
  regenerateExtensionToken,
  type ExtensionToken,
} from "../api/extension-api";

export const extensionKeys = {
  token: ["extension", "token"] as const,
};

export function useExtensionToken() {
  return useQuery<ExtensionToken>({
    queryKey: extensionKeys.token,
    queryFn: getExtensionToken,
    // Only changes when the user regenerates it here.
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });
}

export function useRegenerateExtensionToken() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: regenerateExtensionToken,
    onSuccess: (fresh) => queryClient.setQueryData(extensionKeys.token, fresh),
  });
}
