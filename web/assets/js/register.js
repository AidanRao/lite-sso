document.addEventListener('DOMContentLoaded', function() {
    initRegisterForm();
});

function initRegisterForm() {
    const form = document.getElementById('registerForm');
    const sendOtpBtn = document.getElementById('sendOtp');
    
    if (!form || !sendOtpBtn) return;
    
    sendOtpBtn.addEventListener('click', async () => {
        const email = document.getElementById('email').value;
        const captchaBox = document.getElementById('captchaBox');
        const captcha = captchaBox.getValue();
        const captchaId = captchaBox.getCaptchaId();
        
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
            captchaBox.clear();
        }
    });
    
    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        
        const email = document.getElementById('email').value;
        const password = document.getElementById('password').value;
        const confirmPassword = document.getElementById('confirmPassword').value;
        const username = document.getElementById('username').value;
        const otp = document.getElementById('otp').value;
        
        if (password !== confirmPassword) {
            showError('两次输入的密码不一致');
            return;
        }
        
        if (password.length < 8) {
            showError('密码长度至少8位');
            return;
        }
        
        try {
            const data = await fetchApi('/api/user/register', {
                method: 'POST',
                body: JSON.stringify({
                    email,
                    password,
                    username: username || undefined,
                    otp,
                }),
            });
            showSuccess('注册成功');
            handleAuthSuccess(data);
        } catch (err) {
            showError(err.message);
        }
    });
}

function startCountdown(btn, seconds) {
    btn.disabled = true;
    let remaining = seconds;
    
    const timer = setInterval(() => {
        remaining--;
        btn.textContent = `${remaining}s`;
        
        if (remaining <= 0) {
            clearInterval(timer);
            btn.disabled = false;
            btn.textContent = '发送验证码';
        }
    }, 1000);
}
