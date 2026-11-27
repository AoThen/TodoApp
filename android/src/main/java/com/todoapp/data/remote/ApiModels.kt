package com.todoapp.data.remote

import com.google.gson.annotations.SerializedName
import retrofit2.Response
import retrofit2.http.*
import okhttp3.ResponseBody

interface ApiService {
    @POST("auth/login")
    suspend fun login(@Body request: LoginRequest): Response<LoginResponse>

    @POST("auth/refresh")
    suspend fun refreshToken(): Response<RefreshResponse>

    @POST("auth/logout")
    suspend fun logout(): Response<Unit>

    @GET("tasks")
    suspend fun getTasksPage(
        @Query("page") page: Int,
        @Query("page_size") pageSize: Int
    ): Response<TasksPageResponse>

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

    // Device pairing methods
    @POST("devices/pair/init")
    suspend fun initPairing(
        @Body request: InitPairingRequest
    ): Response<InitPairingResponse>

    @POST("devices/pair")
    suspend fun pairDevice(
        @Body request: PairingRequest
    ): Response<PairingResponse>

    @GET("devices")
    suspend fun getDevices(): Response<DevicesResponse>
}

data class LoginRequest(
    @SerializedName("email") val email: String,
    @SerializedName("password") val password: String
)

data class LoginResponse(
    @SerializedName("access_token") val accessToken: String,
    @SerializedName("refresh_token") val refreshToken: String? = null,
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
    @SerializedName("completed_at") val completedAt: String?,
    @SerializedName("sort_order") val sortOrder: Double? = null,
    @SerializedName("recurrence_type") val recurrenceType: String? = null,
    @SerializedName("recurrence_series_id") val recurrenceSeriesId: String? = null
)

/** GET /tasks 响应：{success, data:{tasks, pagination}} */
data class TasksPageResponse(
    @SerializedName("success") val success: Boolean = false,
    @SerializedName("data") val data: TasksPageData? = null
)

data class TasksPageData(
    @SerializedName("tasks") val tasks: List<TaskResponse> = emptyList(),
    @SerializedName("pagination") val pagination: TasksPagination? = null
)

data class TasksPagination(
    @SerializedName("page") val page: Int = 1,
    @SerializedName("page_size") val pageSize: Int = 20,
    @SerializedName("total") val total: Int = 0,
    @SerializedName("pages") val pages: Int = 1
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
    @SerializedName("due_at") val dueAt: String? = null,
    @SerializedName("sort_order") val sortOrder: Double? = null,
    @SerializedName("recurrence_type") val recurrenceType: String? = null,
    @SerializedName("recurrence_series_id") val recurrenceSeriesId: String? = null
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
    // 服务端任务的 local_id：用于精确匹配本地离线创建的同源任务
    @SerializedName("local_id") val localId: String? = null,
    // 以下字段服务端在 update/delete 场景可能不下发，需可空并在客户端做合并兜底
    @SerializedName("title") val title: String? = null,
    @SerializedName("description") val description: String? = null,
    @SerializedName("status") val status: String? = null,
    @SerializedName("priority") val priority: String? = null,
    @SerializedName("due_at") val dueAt: String? = null,
    @SerializedName("completed_at") val completedAt: String? = null,
    @SerializedName("updated_at") val updatedAt: String,
    @SerializedName("is_deleted") val isDeleted: Boolean = false,
    @SerializedName("sort_order") val sortOrder: Double? = null,
    @SerializedName("recurrence_type") val recurrenceType: String? = null,
    @SerializedName("recurrence_series_id") val recurrenceSeriesId: String? = null
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
    // 服务端冲突响应可能不下发 options，需可空避免 Gson 填 null 触发 NPE
    @SerializedName("options") val options: List<String>? = null
)

// Device pairing data classes
data class PairingRequest(
    @SerializedName("key") val key: String,
    @SerializedName("device_type") val deviceType: String,
    @SerializedName("device_id") val deviceId: String,
    @SerializedName("client_public_key") val clientPublicKey: String
)

data class PairingResponse(
    @SerializedName("status") val status: String,
    @SerializedName("device_id") val deviceId: String?,
    @SerializedName("server_url") val serverUrl: String?,
    @SerializedName("wrapped_key") val wrappedKey: String?,
    @SerializedName("server_public_key") val serverPublicKey: String?,
    @SerializedName("nonce") val nonce: String?,
    @SerializedName("key_exchange") val keyExchange: String?
)

data class InitPairingRequest(
    @SerializedName("device_type") val deviceType: String,
    @SerializedName("device_id") val deviceId: String?
)

data class InitPairingResponse(
    @SerializedName("key") val key: String,
    @SerializedName("device_id") val deviceId: String,
    @SerializedName("server_url") val serverUrl: String,
    @SerializedName("expires_at") val expiresAt: String
)

data class Device(
    @SerializedName("id") val id: Long,
    @SerializedName("device_type") val deviceType: String,
    @SerializedName("device_id") val deviceId: String,
    @SerializedName("server_url") val serverUrl: String,
    @SerializedName("paired_at") val pairedAt: String,
    @SerializedName("last_seen") val lastSeen: String,
    @SerializedName("is_active") val isActive: Boolean
)

data class DevicesResponse(
    @SerializedName("devices") val devices: List<Device>,
    @SerializedName("count") val count: Int
)
