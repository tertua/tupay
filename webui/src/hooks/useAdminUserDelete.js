import { useMutation, useQueryClient } from "@tanstack/react-query";
import { adminUsersApi } from "@/api/adminUsers";
import { adminUsersKey } from "./useAdminUsers";

// Deletes an account without invoice records and refreshes the users table.
export function useDeleteUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id) => adminUsersApi.deleteUser(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: adminUsersKey }),
  });
}
