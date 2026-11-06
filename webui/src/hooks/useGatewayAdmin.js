import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { adminApi } from "@/api/admin";
import { gatewayApi } from "@/api/gateway";

export const gatewayProjectsKey = ["admin", "gateway", "projects"];
export const gatewayTransactionsKey = ["admin", "gateway", "transactions"];
export const gatewayDeliveriesKey = ["admin", "gateway", "deliveries"];
export const gatewayStatusKey = ["gateway", "status"];

export function useGatewayStatus() {
  return useQuery({ queryKey: gatewayStatusKey, queryFn: gatewayApi.status });
}

export function useGatewayProjects() {
  return useQuery({ queryKey: gatewayProjectsKey, queryFn: adminApi.listGatewayProjects });
}

export function useGatewayDeliveries() {
  return useQuery({ queryKey: gatewayDeliveriesKey, queryFn: adminApi.listGatewayDeliveries });
}

function useGatewayMutation(mutationFn, keys) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: () => keys.forEach((key) => queryClient.invalidateQueries({ queryKey: key })),
  });
}

export function useCreateGatewayProject() {
  return useGatewayMutation(adminApi.createGatewayProject, [gatewayProjectsKey]);
}

export function useUpdateGatewayProject() {
  return useGatewayMutation(({ slug, payload }) => adminApi.updateGatewayProject(slug, payload), [gatewayProjectsKey]);
}

export function useRetryGatewayDelivery() {
  return useGatewayMutation(adminApi.retryGatewayDelivery, [gatewayDeliveriesKey]);
}
