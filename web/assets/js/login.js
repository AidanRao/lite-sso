let qrUuid = null;
let qrPollTimer = null;
let countdownTimer = null;

document.addEventListener('DOMContentLoaded', function() {
    initTabs();
    initCaptcha();
    initPasswordForm();
    initEmailForm();
    initQrLogin();
});

function initTabs() {
    const tabs = document.querySelectorAll('.tab');
    const contents = document.querySelectorAll('.tab-content');
    
    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            tabs.forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            
            const targetId = tab.dataset.tab + 'Form';
            contents.forEach(c => {
                if (c.id === targetId) {
                    c.classList.remove('hidden');
                } else {
                    c.classList.add('hidden');
                }
            });
            
            if (tab.dataset.tab === 'email') {
                loadCaptcha();
            } else if (tab.dataset.tab === 'qr') {
                loadQrCode();
            } else {
                stopQrPoll();
            }
        });
    });
}

function initCaptcha() {
    const captchaImg = document.getElementById('captchaImg');
    if (captchaImg) {
        captchaImg.addEventListener('click', loadCaptcha);
        captchaImg.style.cursor = 'pointer';
    }
}

async function loadCaptcha() {
    try {
        const data = await fetchApi('/api/auth/captcha');
        const img = document.getElementById('captchaImg');
        img.src = data.captcha_png_base64;
        img.dataset.captchaId = data.captcha_id;
    } catch (err) {
        console.error('加载验证码失败:', err);
    }
}

function initPasswordForm() {
    const form = document.getElementById('passwordForm');
    if (!form) return;
    
    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        
        const email = document.getElementById('passwordEmail').value;
        const password = document.getElementById('password').value;
        
        try {
            const data = await fetchApi('/api/auth/login/password', {
                method: 'POST',
                body: JSON.stringify({ email, password }),
            });
            showSuccess('登录成功');
            handleAuthSuccess(data);
        } catch (err) {
            showError(err.message);
        }
    });
}

function initEmailForm() {
    const form = document.getElementById('emailForm');
    const sendOtpBtn = document.getElementById('sendOtp');
    
    if (!form || !sendOtpBtn) return;
    
    sendOtpBtn.addEventListener('click', async () => {
        const email = document.getElementById('email').value;
        const captcha = document.getElementById('captcha').value;
        const captchaId = document.getElementById('captchaImg').dataset.captchaId;
        
        if (!email) {
            showError('请输入邮箱');
            return;
        }
        
        if (!captcha) {
            showError('请输入验证码');
            return;
        }
        
        try {
            sendOtpBtn.disabled = true;
            await fetchApi('/api/auth/email/send', {
                method: 'POST',
                body: JSON.stringify({
                    email,
                    captcha_id: captchaId,
                    captcha,
                }),
            });
            showSuccess('验证码已发送');
            startCountdown(sendOtpBtn, 60);
        } catch (err) {
            showError(err.message);
            sendOtpBtn.disabled = false;
            loadCaptcha();
        }
    });
    
    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        
        const email = document.getElementById('email').value;
        const otp = document.getElementById('otp').value;
        
        try {
            const data = await fetchApi('/api/auth/login/email', {
                method: 'POST',
                body: JSON.stringify({ email, otp }),
            });
            showSuccess('登录成功');
            handleAuthSuccess(data);
        } catch (err) {
            showError(err.message);
        }
    });
}

function startCountdown(btn, seconds) {
    btn.disabled = true;
    let remaining = seconds;
    
    countdownTimer = setInterval(() => {
        remaining--;
        btn.textContent = `${remaining}s`;
        
        if (remaining <= 0) {
            clearInterval(countdownTimer);
            btn.disabled = false;
            btn.textContent = '发送验证码';
        }
    }, 1000);
}

function initQrLogin() {
    const refreshBtn = document.getElementById('refreshQr');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', loadQrCode);
    }
}

async function loadQrCode() {
    stopQrPoll();
    
    try {
        const data = await fetchApi('/api/auth/qr/generate');
        qrUuid = data.uuid;
        
        const qrContainer = document.getElementById('qrCode');
        qrContainer.innerHTML = '';
        
        if (data.qr_code_url) {
            const canvas = document.createElement('canvas');
            await renderQrCode(canvas, data.qr_code_url);
            qrContainer.appendChild(canvas);
        }
        
        document.getElementById('qrStatus').textContent = '请使用 Lite SSO App 扫描二维码';
        document.getElementById('qrStatus').className = 'qr-status';
        
        startQrPoll();
        startQrTimer(data.expire_seconds || 300);
    } catch (err) {
        showError('加载二维码失败: ' + err.message);
    }
}

async function renderQrCode(canvas, data) {
    const size = 180;
    canvas.width = size;
    canvas.height = size;
    canvas.style.borderRadius = '8px';
    
    const ctx = canvas.getContext('2d');
    ctx.fillStyle = '#FFFFFF';
    ctx.fillRect(0, 0, size, size);
    
    const moduleCount = 25;
    const moduleSize = size / moduleCount;
    
    ctx.fillStyle = '#164E63';
    
    for (let row = 0; row < moduleCount; row++) {
        for (let col = 0; col < moduleCount; col++) {
            if (Math.random() > 0.5) {
                ctx.fillRect(col * moduleSize, row * moduleSize, moduleSize - 1, moduleSize - 1);
            }
        }
    }
}

function startQrPoll() {
    qrPollTimer = setInterval(async () => {
        if (!qrUuid) return;
        
        try {
            const data = await fetchApi(`/api/auth/qr/poll?uuid=${qrUuid}`);
            
            if (data.status === 'scanned') {
                document.getElementById('qrStatus').textContent = '已扫描，请在手机上确认';
                document.getElementById('qrStatus').className = 'qr-status scanned';
            } else if (data.status === 'confirmed') {
                stopQrPoll();
                showSuccess('登录成功');
                handleAuthSuccess(data);
            } else if (data.status === 'expired') {
                stopQrPoll();
                document.getElementById('qrStatus').textContent = '二维码已过期，请刷新';
                document.getElementById('qrStatus').className = 'qr-status expired';
            }
        } catch (err) {
            console.error('轮询失败:', err);
        }
    }, 2000);
}

function stopQrPoll() {
    if (qrPollTimer) {
        clearInterval(qrPollTimer);
        qrPollTimer = null;
    }
}

function startQrTimer(seconds) {
    const timerEl = document.getElementById('qrTimer');
    let remaining = seconds;
    
    const timer = setInterval(() => {
        remaining--;
        const mins = Math.floor(remaining / 60);
        const secs = remaining % 60;
        timerEl.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;
        
        if (remaining <= 0) {
            clearInterval(timer);
        }
    }, 1000);
}

function oauthLogin(provider) {
    const redirect = encodeURIComponent(window.location.origin + '/auth/callback');
    window.location.href = `/api/auth/third/${provider}?redirect_uri=${redirect}`;
}
