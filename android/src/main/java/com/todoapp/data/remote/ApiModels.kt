package com.todoapp.data.remote

import com.google.gson.annotations.SerializedName
import retrofit2.Response
import retrofit2.http.*

interface ApiService {
    @POST("auth/login")
    suspend fun login(@Body request: LoginRequest): Response<LoginResponse>

    @POST("auth/refresh")
    suspend fun refreshToken(): Response<RefreshResponse>

    @POST("auth/logout")
    suspend fun logout(): Response<Unit>

    @GET("tasks")
    suspend fun getTasks(): Response<List<TaskResponse>>

    @POST("tasks")
    suspend fun createTask(@Body task: CreateTaskRequest): Response<TaskResponse>

    @PATCH("tasks/{id}")
    suspend fun updateTask(
        @Path("id") id: Long,
        @Body task: UpdateTaskRequest
    ): Response<TaskResponse>

    @DELETE("tasks/{id}")
    suspend fun deleteTask(@Path("id") id: Long): Response<Unit>

    @POST("sync")
    suspend fun sync(@Body request: SyncRequest): Response<SyncResponse>

    @GET("export")
    suspend fun exportTasks(
        @Query("type") type: String,
        @Query("format") format: String
    ): Response<ResponseBody>
}

data class LoginRequest(
    @SerializedName("email") val email: String,
    @SerializedName("password") val password: String
)

data class LoginResponse(
    @SerializedName("access_token") val accessToken: String,
    @SerializedName("expires_in") val expiresIn: Int
)

data class RefreshResponse(
    @SerializedName("access_token") val accessToken: String,
    @SerializedName("expires_in") val expiresIn: Int
)

data class TaskResponse(
    @SerializedName("id") val id: Long,
    @SerializedName("title") val title: String,
    @SerializedName("description") val description: String?,
    @SerializedName("status") val status: String,
    @SerializedName("priority") val priority: String?,
    @SerializedName("due_at") val dueAt: String?,
    @SerializedName("created_at") val createdAt: String?,
    @SerializedName("updated_at") val updatedAt: String?,
    @SerializedName("completed_at") val completedAt: String?
)

data class CreateTaskRequest(
    @SerializedName("title") val title: String,
    @SerializedName("description") val description: String? = null,
    @SerializedName("status") val status: String = "todo",
    @SerializedName("priority") val priority: String? = null,
    @SerializedName("due_at") val dueAt: String? = null
)

data class UpdateTaskRequest(
    @SerializedName("title") val title: String? = null,
    @SerializedName("description") val description: String? = null,
    @SerializedName("status") val status: String? = null,
    @SerializedName("priority") val priority: String? = null,
    @SerializedName("due_at") val dueAt: String? = null
)

data class SyncRequest(
    @SerializedName("last_sync_at") val lastSyncAt: String,
    @SerializedName("changes") val changes: List<DeltaChangeRequest>
)

data class DeltaChangeRequest(
    @SerializedName("local_id") val localId: String,
    @SerializedName("op") val op: String,
    @SerializedName("payload") val payload: Map<String, Any>,
    @SerializedName("client_version") val clientVersion: Int
)

data class SyncResponse(
    @SerializedName("server_changes") val serverChanges: List<ServerChange>,
    @SerializedName("client_changes") val clientChanges: List<ClientChange>,
    @SerializedName("last_sync_at") val lastSyncAt: String,
    @SerializedName("conflicts") val conflicts: List<Conflict>
)

data class ServerChange(
    @SerializedName("id") val id: Long,
    @SerializedName("server_version") val serverVersion: Int,
    @SerializedName("title") val title: String,
    @SerializedName("updated_at") val updatedAt: String,
    @SerializedName("is_deleted") val isDeleted: Boolean
)

data class ClientChange(
    @SerializedName("local_id") val localId: String,
    @SerializedName("server_id") val serverId: Long,
    @SerializedName("op") val op: String
)

data class Conflict(
    @SerializedName("local_id") val localId: String,
    @SerializedName("server_id") val serverId: Long,
    @SerializedName("reason") val reason: String,
    @SerializedName("options") val options: List<String>
)
