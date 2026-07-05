from mcp.server.fastmcp import FastMCP
from app.dependencies import (
    get_bocha_client,
    get_oa_client,
    get_travel_service_sync,
    get_vector_db_client,
    get_wecom_client,
)
from app.utils.time_utils import get_current_time, get_current_timestamp, format_timestamp, parse_time_to_timestamp
from app import config

# --- Define MCP Service ---
mcp_server = FastMCP("OA_WeCom_Service")

def _clean_long_fields(data):
    if isinstance(data, dict):
        # Keys to remove directly to save context window
        keys_to_remove = ["raw", "route", "polyline", "web_pages", "images", "cards", "related_questions"]
        cleaned = {k: _clean_long_fields(v) for k, v in data.items() if k not in keys_to_remove}
        
        # Specific cleanup for bocha API messages array if present
        if "messages" in cleaned and isinstance(cleaned["messages"], list):
            cleaned["messages"] = [msg for msg in cleaned["messages"] if isinstance(msg, dict) and msg.get("type") in ["answer", "follow_up"]]
            
        return cleaned
    elif isinstance(data, list):
        return [_clean_long_fields(item) for item in data]
    return data

if "oa" in config.ENABLED_FEATURES:
    @mcp_server.tool()
    def get_oa_base_info() -> dict:
        """
        Get OA Base Info.
        Returns general information about the user's portal, including unread counts and module status.
        """
        return get_oa_client().get_base_info()

    @mcp_server.tool()
    def get_oa_todo_list() -> dict:
        """
        Get OA Todo List.
        Returns a list of pending tasks (待办事宜) from the OA system.
        Output includes task count and details (requestname, requestid, creator, date).
        """
        return get_oa_client().get_todo_info()

if "wecom" in config.ENABLED_FEATURES:
    @mcp_server.tool()
    def get_wecom_departments(id: int = None) -> list:
        """
        Get WeCom Departments.
        :param id: (Optional) Department ID. If omitted, fetches all top-level departments.
        """
        return get_wecom_client().get_departments(id)

    @mcp_server.tool()
    def get_wecom_department_users(department_id: int, fetch_child: int = 0) -> list:
        """
        Get Users in a WeCom Department.
        :param department_id: The ID of the department.
        :param fetch_child: 1 to fetch users recursively from child departments, 0 for direct members only.
        """
        return get_wecom_client().get_department_users(department_id, fetch_child)

    @mcp_server.tool()
    def resolve_wecom_userid(name: str) -> dict:
        """
        Resolve Name to UserID.
        Looks up a UserID by name using the internal address book cache.
        Useful for verifying if a user exists before adding them to a schedule.
        :param name: The full name of the user (e.g., "张三").
        """
        userid = get_wecom_client().get_userid_by_name(name)
        if userid:
            return {"name": name, "userid": userid}
        else:
            return {"error": f"User '{name}' not found"}

    @mcp_server.tool()
    def create_wecom_schedule(summary: str, start_time: int, end_time: int, attendees: list[str], description: str = "", location: str = "") -> dict:
        """
        Create a WeCom Schedule.
        
        :param summary: Schedule title (e.g., "Project Meeting")
        :param start_time: Start time as Unix Timestamp
        :param end_time: End time as Unix Timestamp
        :param attendees: List of names (e.g., ["ZhangSan", "LiSi"]). Will be auto-resolved to UserIDs.
        :param description: Optional description
        :param location: Optional location
        """
        if not summary or not start_time or not end_time:
             return {"error": "Missing required fields: summary, start_time, end_time"}

        try:
            result = get_wecom_client().create_schedule(
                summary=summary,
                start_time=start_time,
                end_time=end_time,
                description=description,
                location=location,
                attendees=attendees
            )
            return result
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def create_wecom_doc(doc_type: int, doc_name: str, admin_users: list[str] = [], spaceid: str = None, fatherid: str = None) -> dict:
        """
        Create a WeCom Document.
        
        :param doc_type: 3: Doc, 4: Sheet, 10: Smart Sheet
        :param doc_name: Document name
        :param admin_users: List of admin names (e.g., ["ZhangSan"]). Will be auto-resolved to UserIDs.
        :param spaceid: Space ID (optional)
        :param fatherid: Parent folder ID (optional)
        """
        try:
            return get_wecom_client().create_doc(doc_type, doc_name, admin_users, spaceid, fatherid)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def rename_wecom_doc(docid: str, new_name: str) -> dict:
        """
        Rename a WeCom Document.
        
        :param docid: Document ID (File ID)
        :param new_name: New name
        """
        try:
            return get_wecom_client().rename_doc(docid, new_name)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def delete_wecom_doc(docid: str) -> dict:
        """
        Delete a WeCom Document.
        
        :param docid: Document ID (File ID)
        """
        try:
            return get_wecom_client().delete_doc(docid)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def list_wecom_docs(spaceid: str = None, fatherid: str = None, sort_type: int = None, start: int = 0, limit: int = 100) -> dict:
        """
        List WeCom Documents.
        
        :param spaceid: Space ID (optional)
        :param fatherid: Parent folder ID (optional)
        :param sort_type: Sort type (1: Name asc, 2: Name desc, 3: Size asc, 4: Size desc, 5: Time asc, 6: Time desc)
        :param start: Offset
        :param limit: Limit
        """
        try:
            return get_wecom_client().list_docs(spaceid, fatherid, sort_type, start, limit)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def get_wecom_doc_info(docid: str) -> dict:
        """
        Get WeCom Document Info.
        
        :param docid: Document ID (File ID)
        """
        try:
            return get_wecom_client().get_doc_info(docid)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def share_wecom_doc(docid: str, users: list[str]) -> dict:
        """
        Share WeCom Document (Add Partners).
        
        :param docid: Document ID
        :param users: List of user names to share with. Will be auto-resolved to UserIDs.
        """
        try:
            return get_wecom_client().add_doc_partner(docid, users)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def get_wecom_doc_content(docid: str) -> dict:
        """
        Get WeCom Document Content.
        
        :param docid: Document ID
        """
        try:
            return get_wecom_client().get_document_content(docid)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def update_wecom_doc_content(docid: str, requests: list[dict], version: int = None) -> dict:
        """
        Update WeCom Document Content (Word-like documents).
        
        :param docid: Document ID
        :param requests: List of update requests (UpdateRequest objects)
        :param version: Document version (optional)
        """
        try:
            return get_wecom_client().update_document_content(docid, requests, version)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def update_wecom_sheet(docid: str, requests: list[dict]) -> dict:
        """
        Batch Update WeCom Sheet (Spreadsheet documents).
        
        This tool requires strict structure for requests.
        Example 1 (Add Sheet): [{"add_sheet_request": {"title": "Sheet2"}}]
        Example 2 (Update Range): [{"update_range_request": {"sheet_id": "xxx", "grid_data": {"start_row": 0, "start_column": 0, "rows": [{"values": [{"cell_value": {"text": "Hello"}}]}]}}}]
        
        Helper for updating simple values (if you provide simplified structure):
        If you provide `{"type": "SetCellValue", "sheet_id": "xxx", "cell": "A1", "value": "val"}`, it will be converted.
        Supported simplified types: "SetCellValue"
        
        :param docid: Document ID
        :param requests: List of update requests
        """
        
        # Pre-process requests to support simplified format
        processed_requests = []
        for req in requests:
            if req.get("type") == "SetCellValue":
                # Convert simplified SetCellValue to update_range_request
                # Req: {"type": "SetCellValue", "sheet_id": "xxx", "cell": "A1", "value": "val"}
                try:
                    import re
                    cell_ref = req.get("cell", "A1")
                    # Parse A1 notation to row/col index (0-based)
                    col_str = re.match(r"([A-Z]+)", cell_ref).group(1)
                    row_str = re.match(r"[A-Z]+(\d+)", cell_ref).group(1)
                    
                    row_idx = int(row_str) - 1
                    col_idx = 0
                    for char in col_str:
                        col_idx = col_idx * 26 + (ord(char) - ord('A') + 1)
                    col_idx -= 1
                    
                    value = req.get("value")
                    processed_requests.append({
                        "update_range_request": {
                            "sheet_id": req.get("sheet_id"),
                            "grid_data": {
                                "start_row": row_idx,
                                "start_column": col_idx,
                                "rows": [{
                                    "values": [{
                                        "cell_value": {
                                            "text": str(value)
                                        }
                                    }]
                                }]
                            }
                        }
                    })
                except Exception as e:
                     return {"error": f"Failed to parse simplified request: {req}, error: {str(e)}"}
            else:
                # Assume it's raw API format
                processed_requests.append(req)

        try:
            return get_wecom_client().sheet_batch_update(docid, processed_requests)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def get_wecom_sheet_properties(docid: str) -> dict:
        """
        Get WeCom Sheet Properties (Row/Col count, etc.).
        
        :param docid: Document ID
        """
        try:
            return get_wecom_client().get_sheet_properties(docid)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def get_wecom_sheet_data(docid: str, sheet_id: str, range: str) -> dict:
        """
        Get WeCom Sheet Range Data.
        
        :param docid: Document ID
        :param sheet_id: Sheet ID
        :param range: Range in A1 notation (e.g., "A1:B2")
        """
        try:
            return get_wecom_client().get_sheet_range_data(docid, sheet_id, range)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def modify_wecom_doc_member(docid: str, update_list: list[dict] = None, delete_list: list[dict] = None) -> dict:
        """
        Modify WeCom Document Member (Add/Update/Delete).
        
        :param docid: Document ID
        :param update_list: List of members to add/update. 
                            Each item: {"userid": "xxx", "auth": 1/2/7, "type": 1}
                            Auth: 1-Readonly, 2-ReadWrite, 7-Admin
        :param delete_list: List of members to delete.
                            Each item: {"userid": "xxx", "type": 1}
        """
        try:
            return get_wecom_client().modify_document_member(docid, update_list, delete_list)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def create_wecom_meeting(title: str, meeting_start: int, meeting_duration: int, description: str = "", location: str = "", invitees: list[str] = [], admin_userid: str = None, remind_before: list[int] = None) -> dict:
        """
        Create a WeCom Meeting.
        
        :param title: Meeting title
        :param meeting_start: Start timestamp (Unix epoch)
        :param meeting_duration: Duration in seconds
        :param description: Description
        :param location: Location
        :param invitees: List of invitee names (e.g., ["ZhangSan", "LiSi"]). Will be auto-resolved to UserIDs.
        :param admin_userid: Admin userid (creator)
        :param remind_before: List of seconds before meeting to remind (e.g. [900] for 15 mins)
        """
        if not title or not meeting_start or not meeting_duration:
             return {"error": "Missing required fields: title, meeting_start, meeting_duration"}

        try:
            return get_wecom_client().create_meeting(
                title=title,
                meeting_start=meeting_start,
                meeting_duration=meeting_duration,
                description=description,
                location=location,
                invitees=invitees,
                admin_userid=admin_userid,
                remind_before=remind_before
            )
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def update_wecom_meeting(meetingid: str, title: str = None, meeting_start: int = None, meeting_duration: int = None, description: str = None, location: str = None, invitees: list[str] = None, remind_before: list[int] = None) -> dict:
        """
        Update a WeCom Meeting.
        
        :param meetingid: Meeting ID
        :param title: New title
        :param meeting_start: New start timestamp
        :param meeting_duration: New duration
        :param description: New description
        :param location: New location
        :param invitees: New invitees list (replaces old list)
        :param remind_before: New remind settings
        """
        try:
            return get_wecom_client().update_meeting(
                meetingid=meetingid,
                title=title,
                meeting_start=meeting_start,
                meeting_duration=meeting_duration,
                description=description,
                location=location,
                invitees=invitees,
                remind_before=remind_before
            )
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def cancel_wecom_meeting(meetingid: str) -> dict:
        """
        Cancel a WeCom Meeting.
        
        :param meetingid: Meeting ID
        """
        try:
            return get_wecom_client().cancel_meeting(meetingid)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def send_wecom_message(touser: str, content: str, msgtype: str = "text", toparty: str = "", totag: str = "") -> dict:
        """
        Send a message to a user or party via WeCom agent.
        Can be used to send text or markdown (e.g. for sending links, notifications).
        
        :param touser: Target user IDs separated by '|'. Use "@all" to send to everyone.
        :param content: Message content (text or markdown format).
        :param msgtype: Type of message: "text" or "markdown".
        :param toparty: Target party IDs separated by '|'.
        :param totag: Target tag IDs separated by '|'.
        """
        try:
            # If the user passed a name, we could try to resolve it.
            # But WeCom send message API requires UserID.
            # Let's check if the touser contains names that we need to resolve.
            # For simplicity, if touser is not "@all" and doesn't contain '|', we can try resolving it if it looks like a name.
            # But WeCom UserIDs can be anything. We will assume the caller provides correct UserIDs or uses resolve_wecom_userid first.
            return get_wecom_client().send_message(touser=touser, content=content, msgtype=msgtype, toparty=toparty, totag=totag)
        except Exception as e:
            return {"error": str(e)}

if "travel" in config.ENABLED_FEATURES:
    @mcp_server.tool()
    def resolve_map_location(target: str, policy: int = 1) -> dict:
        """
        Resolve an address or target place into coordinates.
        :param target: Address or target place description.
        :param policy: Geocoder policy. 0=strict, 1=relaxed.
        """
        try:
            result = get_travel_service_sync().resolve_location(target=target, policy=policy)
            return _clean_long_fields(result)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def search_map_places(
        keyword: str,
        target: str = "",
        boundary: str = "",
        radius: int = 1000,
        auto_extend: int = 1,
        filter_text: str = "",
        page_size: int = 10,
        page_index: int = 1,
    ) -> dict:
        """
        Search places by keyword and target.
        You can either provide boundary directly, or provide target to resolve a center point and search nearby.
        To prevent huge data returns, results are limited to 20 items maximum.
        Use page_index to paginate if there are more results.
        """
        try:
            result = get_travel_service_sync().search_places(
                keyword=keyword,
                target=target or None,
                boundary=boundary or None,
                radius=radius,
                auto_extend=auto_extend,
                filter_text=filter_text or None,
                page_size=page_size,
                page_index=page_index,
            )
            return _clean_long_fields(result)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def search_route_alongby(keyword: str, polyline: str) -> dict:
        """
        Search POIs along a route polyline. Returns a maximum of 20 items.
        :param keyword: Along-route keyword, e.g. 加油站 / 服务区 / 便利店.
        :param polyline: Route polyline string "lat,lng,lat,lng,...".
        """
        try:
            result = get_travel_service_sync().search_along_route(keyword=keyword, polyline=polyline)
            return _clean_long_fields(result)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def enrich_travel_place(
        title: str,
        address: str = "",
        intents: list[str] = None,
        city: str = "",
        category: str = "",
    ) -> dict:
        """
        Enrich place data with score, price, average spend and related information.
        Prioritizes VectorDB recall and falls back to Bocha search when needed.
        """
        try:
            result = get_travel_service_sync().enrich_place(
                title=title,
                address=address,
                intents=intents,
                city=city,
                category=category,
            )
            return _clean_long_fields(result)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def recall_travel_knowledge(query: str, limit: int = 5) -> dict:
        """
        Recall travel knowledge from VectorDB by semantic query.
        """
        try:
            return get_vector_db_client().search_text(query=query, limit=limit)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def store_travel_knowledge(records: list[dict]) -> dict:
        """
        Store normalized travel knowledge records into VectorDB.
        Each record should contain title/text/metadata and optional source fields.
        """
        try:
            return get_vector_db_client().upsert_records(records)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def search_travel_weather(location: str, date_text: str = "") -> dict:
        """
        Search weather information for travel planning via Bocha AI Search.
        """
        try:
            return get_bocha_client().search_weather(location=location, date_text=date_text or None)
        except Exception as e:
            return {"error": str(e)}

    @mcp_server.tool()
    def plan_travel_trip(
        origin: str,
        destination: str,
        via_points: list[str] = None,
        along_keywords: list[str] = None,
        days: int = 1,
        weather_date: str = "",
        enrichment_intents: list[str] = None,
    ) -> dict:
        """
        Generate a travel plan that combines route planning, along-route search, enrichment recall and packing suggestions.
        """
        try:
            result = get_travel_service_sync().plan_trip(
                origin=origin,
                destination=destination,
                via_points=via_points,
                along_keywords=along_keywords,
                days=days,
                weather_date=weather_date or None,
                enrichment_intents=enrichment_intents,
            )
            return _clean_long_fields(result)
        except Exception as e:
            return {"error": str(e)}

# --- Common Tools ---
@mcp_server.tool()
def get_time_current(format_str: str = "%Y-%m-%d %H:%M:%S") -> str:
    """
    Get current time formatted as string.
    :param format_str: The format string (default: "%Y-%m-%d %H:%M:%S")
    """
    return get_current_time(format_str)

@mcp_server.tool()
def get_time_timestamp(is_milliseconds: bool = False) -> int:
    """
    Get current timestamp.
    :param is_milliseconds: Whether to return in milliseconds (default: False)
    """
    return get_current_timestamp(is_milliseconds)

@mcp_server.tool()
def get_time_formatted(timestamp: int, format_str: str = "%Y-%m-%d %H:%M:%S", is_milliseconds: bool = False) -> str:
    """
    Convert timestamp to formatted string.
    :param timestamp: The timestamp to format
    :param format_str: The format string (default: "%Y-%m-%d %H:%M:%S")
    :param is_milliseconds: Whether the input timestamp is in milliseconds (default: False)
    """
    return format_timestamp(timestamp, format_str, is_milliseconds)

@mcp_server.tool()
def get_time_parsed(time_str: str, format_str: str = "%Y-%m-%d %H:%M:%S", is_milliseconds: bool = False) -> int:
    """
    Convert formatted time string to timestamp.
    :param time_str: The time string to parse
    :param format_str: The format string matching the time_str (default: "%Y-%m-%d %H:%M:%S")
    :param is_milliseconds: Whether to return in milliseconds (default: False)
    """
    return parse_time_to_timestamp(time_str, format_str, is_milliseconds)
