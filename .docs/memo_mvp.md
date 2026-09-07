# Memo: Mức độ đáp ứng nghiệp vụ của MVP Solver Xếp Ca

**Đối tượng:** đội phát triển / quản lý vận hành
**Phạm vi:** đánh giá notebook `shift_scheduler_mvp.ipynb` so với bản mô tả nghiệp vụ gốc, để làm cơ sở tinh chỉnh ở các vòng tiếp theo.
**Trạng thái MVP:** solver-only, chạy trên Google Colab, dữ liệu mẫu hardcode, chưa có backend/frontend/DB thật.

---

## 1.5. Cập nhật (sau review): đã sửa phần "thời gian mỗi ca khác nhau theo cổng"

Notebook đã bổ sung `SHIFT_HOURS[gate][shift]` để tính effort/target theo **giờ thật** thay vì đếm số ca — đúng mô tả gốc *"4 cổng có khối lượng công việc khác nhau, thời gian mỗi ca làm khác nhau"*.

**Cập nhật tiếp theo (đã sửa): đơn vị đúng là 44h/TUẦN, không phải 44h/tháng** như hiểu ban đầu. Sau khi sửa:
- Target cho horizon 4 tuần = 44h × 4 = **176h**.
- Tổng nhu cầu tối thiểu 4 cổng ≈ **105 giờ-người/ngày** (~2,940 giờ-người/tháng) → cần tối thiểu **~17 nhân viên** để không ai vượt target.
- Bộ dữ liệu mẫu (21 người) khớp khá sát với con số này: actual hours trung bình ≈ 174h, rất gần target 176h — khác hẳn so với lần chạy trước khi hiểu nhầm là 44h/tháng (lúc đó trung bình lệch tới ~140h so với target 44h).

→ Bài học: chỉ riêng việc chốt sai đơn vị (tuần vs tháng) đã khiến kết quả solve trông như "thiếu nhân sự trầm trọng" trong khi thực ra dữ liệu mẫu đã khá cân đối. Nên rà lại các con số nghiệp vụ khác (độ dài ca thật theo từng cổng, số nhân viên thực tế) theo cùng cách trước khi đưa vào vận hành.

---

## 1.6. Cập nhật (từ bảng phân ca thực tế công ty gửi): số giờ ca THẬT + các phát hiện mới

Bạn đã cung cấp ảnh chụp bảng phân ca tuần thực tế, cho phép thay số ước lượng bằng **số giờ ca thật**:

| Cổng | Ca sáng | Ca đêm |
|---|---|---|
| A | 11h | 13h |
| B | 11h | 12h |
| G | 11h | 13h |
| D | 11h | 12h |

Sau khi cập nhật, capacity check cho kết quả: nhu cầu tối thiểu ≈ **140 giờ-người/ngày** (~3.920 giờ-người/tháng) → cần **~22-23 nhân viên** ở target 44h/tuần. Data mẫu 21 người khá sát, actual hours trung bình ≈187h so với target 176h (lệch ~6%, hợp lý).

**Vài phát hiện thêm từ bảng thật, đã làm rõ qua Q&A và có giá trị cho thiết kế sau này:**
- Mã "N" trong bảng = nhân viên **tự đăng ký không làm** hôm đó (không phải một loại ca) → map thẳng vào `availability=False`, không cần cấu trúc dữ liệu mới.
- Ô tô màu xanh trong bảng thật = ngày bị chặn do **đã làm đêm cuối tuần trước** — xác nhận thực tế công ty đang xử lý continuity giữa các tuần, khớp đúng concern #2 (rolling horizon cần mang theo trạng thái ca đêm cuối kỳ trước sang kỳ sau) đã nêu ở bước trước.
- "P" (phép) và "NS" (nghỉ thai sản) đều xử lý như nhau qua `leave_days` — khớp quyết định trước đó của bạn (không cần phân loại nghỉ).
- Nhóm nhân viên ca hành chính cố định (ghi "8" mỗi ngày, không gắn cổng) **không thuộc phạm vi** bài toán xếp ca 4 cổng này — xác nhận rõ, MVP không cần model nhóm này.
- Ghi chú "2 nhân viên tự đổi lệnh + kiểm tra chéo ở cổng D" — xác nhận không liên quan đến bài toán xếp lịch, bỏ qua.

---

## 1. Bảng đối chiếu nghiệp vụ ↔ mức độ đáp ứng

| # | Nghiệp vụ (theo tài liệu gốc) | Trạng thái | Cách hiện thực trong solver | Cần tinh chỉnh? |
|---|---|:---:|---|---|
| 1 | Phân cấp nhân viên: NV / Trưởng ca (TC) / Phó ca (PC) | ✅ Đã có | Field `role` trên `Employee`, dùng để phân biệt quyền hạn trong constraint | — |
| 2a | 4 cổng A/B/G/D, mỗi cổng/ca có yêu cầu số lượng NV/TC riêng | ✅ Đã có | Dict `REQUIREMENTS[gate][shift]` — số liệu tách rời khỏi logic solver, sửa trực tiếp không cần đụng code | Điền số liệu thực tế thay vì số mẫu trong tài liệu |
| 2b | Số lượng/vai trò yêu cầu có thể thay đổi theo thực tế, không hard-code | ✅ Đã có | Đúng nhờ thiết kế dạng config (2a) | — |
| 2c | Cổng B: luôn có ≥1 trưởng ca; ca đêm bắt buộc đúng TC; ca sáng có thể thay bằng PC | ✅ Đã có | Cờ `lead_mandatory_role` per (gate, shift): `True` → chỉ tính TC; `False` → tính cả TC hoặc PC | Xác nhận lại: sáng có **bắt buộc phải có ai đó ở vị trí lead** hay chỉ "nên có"? Hiện đang code là bắt buộc (có slack nếu thiếu) |
| 2d | Trưởng ca chỉ làm việc ở cổng B | ✅ Đã có | Hard constraint: biến assignment = 0 với mọi cổng ≠ B nếu `role == TC` | Xác nhận PC có bị giới hạn cổng tương tự không (hiện PC đang được phép làm mọi cổng) |
| 3 | PC là nhân viên kỹ năng cao, backup TC khi không còn TC trống lịch | ✅ Đã có (một phần) | PC được tính vào lead-slot khi `lead_mandatory_role=False`; PC **không** tự động được ưu tiên hơn NV thường ở các cổng khác | Solver hiện chưa "ưu tiên xếp PC dự phòng" khi TC còn đủ — có thể thêm ràng buộc mềm nếu muốn PC luôn có mặt như backup ngay cả khi chưa cần |
| 4a | Yêu cầu ~44 giờ/**tuần**, 3-4 ca/tuần | ✅ Đã sửa, dùng số thật | `SHIFT_HOURS[gate][shift]` nay là **số giờ thật** lấy từ bảng phân ca thực tế (A/G: 11h/13h; B/D: 11h/12h); target = 44h/tuần × số tuần | — |
| 4b | Cơ chế bù trừ giữa các tuần (thiếu tuần này, bù tuần sau) | ⚠️ Một phần | Chỉ cân bằng **tổng cả tháng** (deviation penalty theo target tháng), chưa có ràng buộc riêng cho từng tuần (vd: min/max ca mỗi tuần) | Nếu muốn giới hạn "không quá X ca/tuần dù tổng tháng đủ", cần thêm constraint theo từng tuần |
| 4c | Không xếp ca sáng ngay sau ca đêm hôm trước | ✅ Đã có | Hard constraint: `night[d] + morning[d+1] <= 1` | — |
| 4d | Tránh full ca đêm / full ca sáng liên tục, trừ khi có yêu cầu riêng; ưu tiên xen kẽ | ✅ Đã có (một phần) | Soft constraint: phạt khi có streak 3 ngày liên tiếp cùng loại ca | Ngưỡng "3 ngày" là tự chọn, có thể chỉnh; **chưa có** cơ chế "trừ khi đó là yêu cầu của nhân viên/quản lý" (chưa có override thủ công cho từng người) |
| 5 | Đăng ký ca hàng tuần trước tuần làm việc | ❌ Chưa có | MVP tạo sẵn availability cho **cả tháng cùng lúc** bằng random, không mô phỏng quy trình đăng ký từng tuần | Cần thiết kế lại: hoặc chia solver thành chạy theo từng tuần (rolling horizon), hoặc giữ nguyên chạy cả tháng nhưng data nhập theo tuần |
| 6 | Phân phối công việc đều, tránh lệch quá nhiều giữa các nhân viên | ✅ Đã có | Soft constraint: phạt độ lệch giữa số ca thực tế và target cá nhân (`BALANCE_PENALTY_WEIGHT`) | Trọng số hiện đặt thấp hơn shortfall — nếu quản lý muốn ưu tiên công bằng hơn cả việc đủ người, cần tăng trọng số này |
| 7 | Đảm bảo min-staffing; nếu không đủ → yêu cầu manual review | ✅ Đã có | Biến slack (`short`, `lead_short`) cho phép thiếu người nhưng bị phạt nặng trong objective; danh sách thiếu hụt xuất ra `shortage_df` | Cần định nghĩa quy trình: khi có dòng trong `shortage_df`, quản lý sẽ làm gì tiếp theo (UI review chưa có) |
| 8 | Hiển thị bảng dạng excel để dễ migrate | ✅ Đã có | `schedule_df` (pandas DataFrame: hàng = nhân viên, cột = ngày, giá trị = `Cổng-Ca`/`OFF`) | Format cột/màu sắc cụ thể cần làm ở tầng UI thật (không thuộc solver) |
| 9a | Đổi ca / xử lý sự cố đột xuất, có tuần làm 2 ngày tuần sau bù 4 ngày | ⚠️ Một phần | Demo re-solve cục bộ: khóa toàn bộ lịch đã approve, chỉ mở lại đúng ngày bị ảnh hưởng | Đây là mô phỏng **1 ngày bị ảnh hưởng**; kịch bản "tuần này 2 ngày, tuần sau bù 4 ngày" cần test case riêng để xác nhận cân bằng tháng vẫn đúng |
| 9b | App tính toán & hiển thị **danh sách nhân viên phù hợp để thay thế**, quản lý manual review/approve | ❌ Chưa có | Hiện solver **tự chọn luôn** người thay thế tối ưu, không xuất ra danh sách ứng viên phù hợp để quản lý chọn | Cần thêm: chạy solver ở chế độ "liệt kê top-N người khả dụng + điểm phù hợp" thay vì chỉ ra 1 kết quả duy nhất |
| 9c | Đổi ca không được ảnh hưởng lịch của nhân viên khác đã approve | ✅ Đã có, đã test | Cơ chế `locked` + test tự động xác nhận **0 ô thay đổi ngoài phạm vi ảnh hưởng** | — |
| 10 | Nghỉ theo luật (ốm, thai sản, phép, chế độ, không lương...) không bị ép bù đủ ca | ✅ Đã có | `leave_days` trên `Employee` → giảm target theo tỷ lệ ngày available, không có shortfall penalty cho cá nhân đó | Hiện chưa phân biệt **loại nghỉ** (ốm/thai sản/phép/không lương) — tất cả xử lý như nhau; nếu cần quy tắc khác nhau theo loại nghỉ (vd: thai sản không tính vào cân bằng tuần kế tiếp) thì cần mở rộng |
| 11 | Quản lý approve từng ngày; lịch đã approve solver không được ghi đè; muốn solve lại toàn bộ cần confirm | ⚠️ Một phần | Cơ chế khóa (`locked` dict) đã hoạt động đúng ở mức kỹ thuật | Chưa có: (a) nơi lưu trạng thái approve persistent, (b) bước "confirm" của quản lý trước khi solve lại toàn bộ — hiện chỉ là gọi hàm Python trực tiếp |
| 12 | Kiến trúc: OR-Tools + FastAPI + Golang backend + React frontend | ❌ Ngoài phạm vi MVP | MVP chỉ có phần solver Python thuần trong notebook | Đã thống nhất từ đầu — để lại cho vòng phát triển sau |

**Chú thích:** ✅ Đã có · ⚠️ Một phần / cần làm rõ thêm · ❌ Chưa có trong MVP này

---

## 2. Tham số có thể tinh chỉnh ngay (không cần sửa logic)

Tất cả nằm ở đầu notebook (mục "Config nghiệp vụ"):

| Tham số | Ý nghĩa | Giá trị hiện tại |
|---|---|---|
| `REQUIREMENTS` | Số lượng NV/TC/PC tối thiểu theo từng cổng/ca | Theo số mẫu trong tài liệu gốc |
| `LEAD_GATES` | Cổng nào có khái niệm "lead slot" | `{"B"}` |
| `BASE_TARGET_SHIFTS_PER_MONTH` | Target số ca/tháng cho 1 nhân viên full-time | `16` |
| `SHORTFALL_PENALTY` | Mức phạt khi thiếu người ở 1 ca | `1000` |
| `LEAD_SHORTFALL_PENALTY` | Mức phạt khi thiếu trưởng ca/phó ca | `800` |
| `BALANCE_PENALTY_WEIGHT` | Mức phạt lệch effort giữa các nhân viên | `5` |
| `STREAK_PENALTY_WEIGHT` | Mức phạt xếp 3 ngày liên tiếp cùng loại ca | `2` |
| Độ dài streak bị phạt (hard-code `3` trong code) | Số ngày liên tiếp cùng ca trước khi bị phạt | `3 ngày` |

Thứ tự độ lớn hiện tại (`Shortfall > Lead shortfall > Balance > Streak`) phản ánh ưu tiên: **đủ người trước, công bằng sau, xen kẽ ca là mong muốn nhẹ nhất**. Nếu quản lý có ưu tiên khác, chỉ cần đổi số, không cần sửa cấu trúc constraint.

---

## 3. Khoảng trống chính cần quyết định trước khi build tiếp

1. **Đơn vị effort:** đã chốt **44h/tuần**, và độ dài ca thật theo từng cổng đã có (A/G: 11h/13h; B/D: 11h/12h) — mục này coi như xong.
2. **Quy trình đăng ký theo tuần:** solver hiện giải nguyên tháng 1 lần; nếu nhân viên đăng ký ca theo từng tuần thực tế, cần thiết kế lại thành rolling horizon (giải từng tuần, có ràng buộc "nối" với tuần trước).
3. **Danh sách ứng viên thay thế:** hiện solver tự quyết định người thay thế; nếu quy trình thực tế cần quản lý **chọn tay** từ danh sách gợi ý, cần thêm bước xuất top-N ứng viên kèm lý do phù hợp (đủ điều kiện, ít ảnh hưởng cân bằng nhất, v.v.).
4. **Workflow approve thật:** MVP mô phỏng khóa bằng dict Python; hệ thống thật cần bảng trạng thái approve (per ngày/nhân viên) + bước xác nhận của quản lý trước khi cho phép solve lại toàn bộ.
5. **Phân loại nghỉ phép:** nếu các loại nghỉ (ốm/thai sản/phép năm/không lương) cần xử lý khác nhau (ví dụ ảnh hưởng khác nhau đến tính lương, đến cơ chế bù trừ), cần mở rộng field `leave_days` thành có loại nghỉ đi kèm.

---

## 4. Đề xuất bước tiếp theo

- Review bảng ở mục 1 cùng quản lý vận hành, đánh dấu mục nào là **must-have cho bản kế tiếp** vs **có thể để sau**.
- Với các mục ✅, dùng chính notebook này để chạy thử với **số liệu thực tế** (thay `REQUIREMENTS`, số nhân viên, target giờ) để kiểm tra kết quả có hợp lý không.
- Với các mục ⚠️/❌, ưu tiên theo mức độ ảnh hưởng vận hành trước khi đầu tư build backend/frontend thật.
