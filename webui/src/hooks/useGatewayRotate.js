import { useMutation, useQueryClient } from "@tanstack/react-query";
import { adminGatewayApi } from "@/api/adminGateway";
import { gatewayProjectsKey } from "@/hooks/useGatewayAdmin";

// Rotations are step-up guarded: the caller must first mint a single-use token
// (StepUpPrompt) and pass it in, so these live outside useGatewayAdmin's
// shared mutation helper.
function useRotateMutation(mutationFn) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: gatewayProjectsKey }),
  });
}

export function useRotateGatewayKey() {
  return useRotateMutation(({ slug, token }) => adminGatewayApi.rotateGatewayKey(slug, token));
}

export function useRotateGatewaySecret() {
  return useRotateMutation(({ slug, token }) => adminGatewayApi.rotateGatewaySecret(slug, token));
}
